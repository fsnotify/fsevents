package fsevents

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// EventFlags extensions for tests.

var eventFlagsPossible = map[string]EventFlags{
	"MustScanSubDirs":   MustScanSubDirs,
	"KernelDropped":     KernelDropped,
	"UserDropped":       UserDropped,
	"EventIDsWrapped":   EventIDsWrapped,
	"HistoryDone":       HistoryDone,
	"RootChanged":       RootChanged,
	"Mount":             Mount,
	"Unmount":           Unmount,
	"ItemCreated":       ItemCreated,
	"ItemRemoved":       ItemRemoved,
	"ItemInodeMetaMod":  ItemInodeMetaMod,
	"ItemRenamed":       ItemRenamed,
	"ItemModified":      ItemModified,
	"ItemFinderInfoMod": ItemFinderInfoMod,
	"ItemChangeOwner":   ItemChangeOwner,
	"ItemXattrMod":      ItemXattrMod,
	"ItemIsFile":        ItemIsFile,
	"ItemIsDir":         ItemIsDir,
	"ItemIsSymlink":     ItemIsSymlink,
}

func (flags EventFlags) Set(mask EventFlags) EventFlags {
	return flags | mask
}

func (flags EventFlags) HasFlag(mask EventFlags) bool {
	return flags&mask != 0
}

func (flags EventFlags) SetFlags() []string {
	var result []string

	for k, f := range eventFlagsPossible {
		if flags.HasFlag(f) {
			result = append(result, k)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i] < result[j]
	})

	return result
}

func (flags EventFlags) String() string {
	setFlags := flags.SetFlags()
	return strings.Join(setFlags, "|")
}

// We wait a little bit after most commands; gives the system some time to sync
// things and makes things more consistent across platforms.
func eventSeparator() { time.Sleep(100 * time.Millisecond) }
func waitForEvents()  { time.Sleep(500 * time.Millisecond) }

// addWatch adds a watch for a directory
func (w *eventCollector) addWatch(t *testing.T, path ...string) {
	t.Helper()
	if len(path) < 1 {
		t.Fatalf("addWatch: path must have at least one element: %s", path)
	}

	p := join(path...)

	if _, found := w.streams[p]; found {
		w.streams[p].Paths = append(w.streams[p].Paths, p)
		fmt.Printf("HERE: %+v\n", w.streams[p].Paths)
		return
	}

	dev, err := DeviceForPath(p)
	if err != nil {
		t.Fatal(err)
	}

	es := &EventStream{
		Paths:   []string{p},
		Latency: 0, // 500 * time.Millisecond,
		Device:  dev,
		Flags:   FileEvents | NoDefer,
	}

	w.streams[p] = es

	if err := w.streams[p].Start(); err != nil {
		t.Fatalf("failed to start event stream: %s", err.Error())
	}

	go func() {
		for msg := range es.Events {
			w.mu.Lock()
			w.e = append(w.e, msg...)
			w.mu.Unlock()
		}
	}()
}

// rmWatch removes a watch.
func (w *eventCollector) rmWatch(t *testing.T, path ...string) {
	t.Helper()
	if len(path) < 1 {
		t.Fatalf("rmWatch: path must have at least one element: %s", path)
	}

	p := join(path...)
	w.streams[p].Flush(true)
	w.streams[p].Stop()
	delete(w.streams, p)
}

func shouldWait(path ...string) bool {
	// Take advantage of the fact that join skips empty parameters.
	for _, p := range path {
		if p == "" {
			return false
		}
	}
	return true
}

// mkdir
func mkdir(t *testing.T, path ...string) {
	t.Helper()
	if len(path) < 1 {
		t.Fatalf("mkdir: path must have at least one element: %s", path)
	}
	err := os.Mkdir(join(path...), 0o0755)
	if err != nil {
		t.Fatalf("mkdir(%q): %s", join(path...), err)
	}
	if shouldWait(path...) {
		eventSeparator()
	}
}

// mkdir -p
func mkdirAll(t *testing.T, path ...string) {
	t.Helper()
	if len(path) < 1 {
		t.Fatalf("mkdirAll: path must have at least one element: %s", path)
	}
	err := os.MkdirAll(join(path...), 0o0755)
	if err != nil {
		t.Fatalf("mkdirAll(%q): %s", join(path...), err)
	}
	if shouldWait(path...) {
		eventSeparator()
	}
}

// ln -s
func symlink(t *testing.T, target string, link ...string) {
	t.Helper()
	if len(link) < 1 {
		t.Fatalf("symlink: link must have at least one element: %s", link)
	}
	err := os.Symlink(target, join(link...))
	if err != nil {
		t.Fatalf("symlink(%q, %q): %s", target, join(link...), err)
	}
	if shouldWait(link...) {
		eventSeparator()
	}
}

// echoAppend and echoTrunc
func echoAppend(t *testing.T, data string, path ...string) { t.Helper(); echo(t, false, data, path...) }
func echoTrunc(t *testing.T, data string, path ...string)  { t.Helper(); echo(t, true, data, path...) }
func echo(t *testing.T, trunc bool, data string, path ...string) {
	n := "echoAppend"
	if trunc {
		n = "echoTrunc"
	}
	t.Helper()
	if len(path) < 1 {
		t.Fatalf("%s: path must have at least one element: %s", n, path)
	}

	err := func() error {
		var (
			fp  *os.File
			err error
		)
		if trunc {
			fp, err = os.Create(join(path...))
		} else {
			fp, err = os.OpenFile(join(path...), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		}
		if err != nil {
			return err
		}
		if err := fp.Sync(); err != nil {
			return err
		}
		if shouldWait(path...) {
			eventSeparator()
		}
		if _, err := fp.WriteString(data); err != nil {
			return err
		}
		if err := fp.Sync(); err != nil {
			return err
		}
		if shouldWait(path...) {
			eventSeparator()
		}
		return fp.Close()
	}()
	if err != nil {
		t.Fatalf("%s(%q): %s", n, join(path...), err)
	}
}

// touch
func touch(t *testing.T, path ...string) {
	t.Helper()
	if len(path) < 1 {
		t.Fatalf("touch: path must have at least one element: %s", path)
	}
	fp, err := os.Create(join(path...))
	if err != nil {
		t.Fatalf("touch(%q): %s", join(path...), err)
	}
	err = fp.Close()
	if err != nil {
		t.Fatalf("touch(%q): %s", join(path...), err)
	}
	if shouldWait(path...) {
		eventSeparator()
	}
}

// mv
func mv(t *testing.T, src string, dst ...string) {
	t.Helper()
	if len(dst) < 1 {
		t.Fatalf("mv: dst must have at least one element: %s", dst)
	}

	err := os.Rename(src, join(dst...))
	if err != nil {
		t.Fatalf("mv(%q, %q): %s", src, join(dst...), err)
	}
	if shouldWait(dst...) {
		eventSeparator()
	}
}

// rm
func rm(t *testing.T, path ...string) {
	t.Helper()
	if len(path) < 1 {
		t.Fatalf("rm: path must have at least one element: %s", path)
	}
	err := os.Remove(join(path...))
	if err != nil {
		t.Fatalf("rm(%q): %s", join(path...), err)
	}
	if shouldWait(path...) {
		eventSeparator()
	}
}

// rm -r
func rmAll(t *testing.T, path ...string) {
	t.Helper()
	if len(path) < 1 {
		t.Fatalf("rmAll: path must have at least one element: %s", path)
	}
	err := os.RemoveAll(join(path...))
	if err != nil {
		t.Fatalf("rmAll(%q): %s", join(path...), err)
	}
	if shouldWait(path...) {
		eventSeparator()
	}
}

// cat
func cat(t *testing.T, path ...string) {
	t.Helper()
	if len(path) < 1 {
		t.Fatalf("cat: path must have at least one element: %s", path)
	}
	_, err := os.ReadFile(join(path...))
	if err != nil {
		t.Fatalf("cat(%q): %s", join(path...), err)
	}
	if shouldWait(path...) {
		eventSeparator()
	}
}

// chmod
func chmod(t *testing.T, mode fs.FileMode, path ...string) {
	t.Helper()
	if len(path) < 1 {
		t.Fatalf("chmod: path must have at least one element: %s", path)
	}
	err := os.Chmod(join(path...), mode)
	if err != nil {
		t.Fatalf("chmod(%q): %s", join(path...), err)
	}
	if shouldWait(path...) {
		eventSeparator()
	}
}

// Collect all events in an array.
//
// w := newCollector(t)
// w.collect(r)
//
// .. do stuff ..
//
// events := w.stop(t)
type eventCollector struct {
	streams map[string]*EventStream
	e       Events
	mu      sync.Mutex
	done    chan struct{}
}

func newCollector() *eventCollector {
	return &eventCollector{
		streams: make(map[string]*EventStream),
		done:    make(chan struct{}),
		e:       make(Events, 0, 8),
	}
}

// stop collecting events and return what we've got.
func (w *eventCollector) stop() Events {
	return w.stopWait(time.Second)
}

func (w *eventCollector) stopWait(waitFor time.Duration) Events {
	waitForEvents()

	time.Sleep(waitFor)

	for _, es := range w.streams {
		es.Flush(true)
		es.Stop()
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	return w.e
}

type Events []Event

func (e Events) String() string {
	b := new(strings.Builder)
	for i, ee := range e {
		if i > 0 {
			b.WriteString("\n")
		}
		//if ee.renamedFrom != "" {
		//	fmt.Fprintf(b, "%-8s %s ← %s", ee.Op.String(), filepath.ToSlash(ee.Name), filepath.ToSlash(ee.renamedFrom))
		//} else {
		//fmt.Fprintf(b, "%-8d %s", ee.Flags, filepath.ToSlash(ee.Path))

		//}

		fmt.Fprintf(b, "%-8v %s", ee.Flags.String(), filepath.ToSlash(ee.Path))
	}
	return b.String()
}

func (e Events) TrimPrefix(prefix string) Events {
	prefix = strings.TrimPrefix(prefix, "/")

	for i := range e {
		if e[i].Path == prefix {
			e[i].Path = "/"
		} else {
			e[i].Path = strings.TrimPrefix(e[i].Path, prefix)
		}
		//if e[i].renamedFrom == prefix {
		//	e[i].renamedFrom = "/"
		//} else {
		//	e[i].renamedFrom = strings.TrimPrefix(e[i].renamedFrom, prefix)
		//}
	}
	return e
}

func (e Events) copy() Events {
	cp := make(Events, len(e))
	copy(cp, e)
	return cp
}

// Create a new Events list from a string; for example:
//
//	CREATE        path
//	CREATE|WRITE  path
//
// Every event is one line, and any whitespace between the event and path are
// ignored. The path can optionally be surrounded in ". Anything after a "#" is
// ignored.
//
// Platform-specific tests can be added after GOOS:
//
//	# Tested if nothing else matches
//	CREATE   path
//
//	# Windows-specific test.
//	windows:
//	  WRITE    path
//
// You can specify multiple platforms with a comma (e.g. "windows, linux:").
// "kqueue" is a shortcut for all kqueue systems (BSD, macOS).
func newEvents(t *testing.T, s string) Events {
	t.Helper()

	var (
		lines  = strings.Split(s, "\n")
		groups = []string{""}
		events = make(map[string]Events)
	)
	for no, line := range lines {
		if i := strings.IndexByte(line, '#'); i > -1 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasSuffix(line, ":") {
			groups = strings.Split(strings.TrimRight(line, ":"), ",")
			for i := range groups {
				groups[i] = strings.TrimSpace(groups[i])
			}
			continue
		}

		fields := strings.Fields(line)
		if len(fields) != 2 && len(fields) != 4 {
			if strings.ToLower(fields[0]) == "empty" || strings.ToLower(fields[0]) == "no-events" {
				for _, g := range groups {
					events[g] = Events{}
				}
				continue
			}
			t.Fatalf("newEvents: line %d: needs 2 or 4 fields: %s", no+1, line)
		}

		var resultFlags EventFlags

		flags := strings.Split(fields[0], "|")
		for _, f := range flags {
			resultFlags = resultFlags.Set(eventFlagsPossible[f])
		}

		for _, g := range groups {
			e := Event{
				Path:  strings.Trim(fields[1], `"`),
				Flags: resultFlags,
			}
			events[g] = append(events[g], e)
		}
	}

	if e, ok := events[runtime.GOOS]; ok {
		return e
	}
	switch runtime.GOOS {
	// kqueue shortcut
	case "freebsd", "netbsd", "openbsd", "dragonfly", "darwin":
		if e, ok := events["kqueue"]; ok {
			return e
		}
	}
	return events[""]
}

func cmpEvents(t *testing.T, tmp string, have, want Events) {
	t.Helper()

	have = have.TrimPrefix(tmp)

	haveSort, wantSort := have.copy(), want.copy()
	sort.Slice(haveSort, func(i, j int) bool {
		return haveSort[i].Path > haveSort[j].Path
	})
	sort.Slice(wantSort, func(i, j int) bool {
		return wantSort[i].Path > wantSort[j].Path
	})

	if haveSort.String() != wantSort.String() {
		//t.Error("\n" + ztest.Diff(indent(haveSort), indent(wantSort)))
		t.Errorf("\nhave:\n%s\nwant:\n%s", indent(have), indent(want))
	}
}

func indent(s fmt.Stringer) string {
	return "\t" + strings.ReplaceAll(s.String(), "\n", "\n\t")
}

var join = filepath.Join

func tmppath(tmp, s string) string {
	if len(s) == 0 {
		return ""
	}
	if !strings.HasPrefix(s, "./") {
		return filepath.Join(tmp, s)
	}
	// Needed for creating relative links. Support that only with explicit "./"
	// – otherwise too easy to forget leading "/" and create files outside of
	// the tmp dir.
	return s
}

type command struct {
	line int
	cmd  string
	args []string
}

func parseScript(t *testing.T, in string) {
	var (
		lines = strings.Split(in, "\n")
		cmds  = make([]command, 0, 8)
		readW bool
		want  string
		tmp   = t.TempDir()
		err   error
	)

	tmp, err = filepath.EvalSymlinks(tmp)
	if err != nil {
		t.Fatalf("evalSymlinks: %v", err)
	}

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line[0] == '#' {
			continue
		}
		if i := strings.IndexByte(line, '#'); i > -1 {
			line = strings.TrimSpace(line[:i])
		}
		if line == "Output:" {
			readW = true
			continue
		}
		if readW {
			want += line + "\n"
			continue
		}

		cmd := command{line: i + 1, args: make([]string, 0, 4)}
		var (
			q   bool
			cur = make([]rune, 0, 16)
			app = func() {
				if len(cur) == 0 {
					return
				}
				if cmd.cmd == "" {
					cmd.cmd = string(cur)
				} else {
					cmd.args = append(cmd.args, string(cur))
				}
				cur = cur[:0]
			}
		)
		for _, c := range line {
			switch c {
			case ' ', '\t':
				if q {
					cur = append(cur, c)
				} else {
					app()
				}
			case '"', '\'': // '
				q = !q
			default:
				cur = append(cur, c)
			}
		}
		app()
		cmds = append(cmds, cmd)
	}

	var (
		do      = make([]func(), 0, len(cmds))
		w       = newCollector()
		mustArg = func(c command, n int) {
			if len(c.args) != n {
				t.Fatalf("line %d: %q requires exactly %d argument (have %d: %q)",
					c.line, c.cmd, n, len(c.args), c.args)
			}
		}
	)
loop:
	for _, c := range cmds {
		c := c
		switch c.cmd {
		case "skip", "require":
			mustArg(c, 1)
			switch c.args[0] {
			case "always":
				t.Skip()
			case "symlink":
				//if !internal.HasPrivilegesForSymlink() {
				//    t.Skipf("%s symlink: admin permissions required on Windows", c.cmd)
				//}
			case "recurse":
				// noop - fsevents works with recurse by default
			case "windows":
				if runtime.GOOS == "windows" {
					t.Skip("Skipping on Windows")
				}
			case "netbsd":
				if runtime.GOOS == "netbsd" {
					t.Skip("Skipping on NetBSD")
				}
			case "openbsd":
				if runtime.GOOS == "openbsd" {
					t.Skip("Skipping on OpenBSD")
				}
			default:
				t.Fatalf("line %d: unknown %s reason: %q", c.line, c.cmd, c.args[0])
			}
		//case "state":
		//	mustArg(c, 0)
		//	do = append(do, func() { eventSeparator(); fmt.Fprintln(os.Stderr); w.w.state(); fmt.Fprintln(os.Stderr) })
		case "debug":
			mustArg(c, 1)
			switch c.args[0] {
			case "1", "on", "true", "yes":
				//do = append(do, func() { debug = true })
			case "0", "off", "false", "no":
				//do = append(do, func() { debug = false })
			default:
				t.Fatalf("line %d: unknown debug: %q", c.line, c.args[0])
			}
		case "stop":
			mustArg(c, 0)
			break loop
		case "watch":
			if len(c.args) < 1 {
				t.Fatalf("line %d: %q requires at least %d arguments (have %d: %q)",
					c.line, c.cmd, 1, len(c.args), c.args)
			}

			do = append(do, func() { w.addWatch(t, tmppath(tmp, c.args[0])) })
		case "unwatch":
			mustArg(c, 1)
			do = append(do, func() { w.rmWatch(t, tmppath(tmp, c.args[0])) })
		case "watchlist":
			mustArg(c, 1)
			n, err := strconv.ParseInt(c.args[0], 10, 0)
			if err != nil {
				t.Fatalf("line %d: %s", c.line, err)
			}
			do = append(do, func() {
				var wl []string
				for _, s := range w.streams {
					wl = append(wl, s.Paths...)
				}
				if l := int64(len(wl)); l != n {
					t.Errorf("line %d: watchlist has %d entries, not %d\n%q", c.line, l, n, wl)
				}
			})
		case "touch":
			mustArg(c, 1)
			do = append(do, func() { touch(t, tmppath(tmp, c.args[0])) })
		case "mkdir":
			recur := false
			if len(c.args) == 2 && c.args[0] == "-p" {
				recur, c.args = true, c.args[1:]
			}
			mustArg(c, 1)
			if recur {
				do = append(do, func() { mkdirAll(t, tmppath(tmp, c.args[0])) })
			} else {
				do = append(do, func() { mkdir(t, tmppath(tmp, c.args[0])) })
			}
		case "ln":
			mustArg(c, 3)
			if c.args[0] != "-s" {
				t.Fatalf("line %d: only ln -s is supported", c.line)
			}
			do = append(do, func() { symlink(t, tmppath(tmp, c.args[1]), tmppath(tmp, c.args[2])) })
		case "mv":
			mustArg(c, 2)
			do = append(do, func() { mv(t, tmppath(tmp, c.args[0]), tmppath(tmp, c.args[1])) })
		case "rm":
			recur := false
			if len(c.args) == 2 && c.args[0] == "-r" {
				recur, c.args = true, c.args[1:]
			}
			mustArg(c, 1)
			if recur {
				do = append(do, func() { rmAll(t, tmppath(tmp, c.args[0])) })
			} else {
				do = append(do, func() { rm(t, tmppath(tmp, c.args[0])) })
			}
		case "chmod":
			mustArg(c, 2)
			n, err := strconv.ParseUint(c.args[0], 8, 32)
			if err != nil {
				t.Fatalf("line %d: %s", c.line, err)
			}
			do = append(do, func() { chmod(t, fs.FileMode(n), tmppath(tmp, c.args[1])) })
		case "cat":
			mustArg(c, 1)
			do = append(do, func() { cat(t, tmppath(tmp, c.args[0])) })
		case "echo":
			if len(c.args) < 2 || len(c.args) > 3 {
				t.Fatalf("line %d: %q requires 2 or 3 arguments (have %d: %q)",
					c.line, c.cmd, len(c.args), c.args)
			}

			var data, op, dst string
			if len(c.args) == 2 { // echo foo >dst
				data, op, dst = c.args[0], c.args[1][:1], c.args[1][1:]
				if strings.HasPrefix(dst, ">") {
					op, dst = op+dst[:1], dst[1:]
				}
			} else { // echo foo > dst
				data, op, dst = c.args[0], c.args[1], c.args[2]
			}

			switch op {
			case ">":
				do = append(do, func() { echoTrunc(t, data, tmppath(tmp, dst)) })
			case ">>":
				do = append(do, func() { echoAppend(t, data, tmppath(tmp, dst)) })
			default:
				t.Fatalf("line %d: echo requires > (truncate) or >> (append): echo data >file", c.line)
			}
		case "sleep":
			mustArg(c, 1)
			n, err := strconv.ParseInt(strings.TrimRight(c.args[0], "ms"), 10, 0)
			if err != nil {
				t.Fatalf("line %d: %s", c.line, err)
			}
			do = append(do, func() { time.Sleep(time.Duration(n) * time.Millisecond) })
		default:
			t.Errorf("line %d: unknown command %q", c.line, c.cmd)
		}
	}

	for _, d := range do {
		eventSeparator()
		d()
	}
	ev := w.stop()
	cmpEvents(t, tmp, ev, newEvents(t, want))
}
