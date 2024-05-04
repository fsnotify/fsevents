//go:build darwin

package fsevents

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestScript(t *testing.T) {
	err := filepath.Walk("./testdata", func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		n := strings.Split(filepath.ToSlash(path), "/")
		t.Run(strings.Join(n[1:], "/"), func(t *testing.T) {
			t.Parallel()

			d, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}

			parseScript(t, string(d))
		})

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestBasicExample(t *testing.T) {
	path, err := os.MkdirTemp("", "fsexample")
	if err != nil {
		t.Fatal(err)
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(path)

	dev, err := DeviceForPath(path)
	if err != nil {
		t.Fatal(err)
	}

	es := &EventStream{
		Paths:   []string{path},
		Latency: 500 * time.Millisecond,
		Device:  dev,
		Flags:   FileEvents,
	}

	err = es.Start()
	if err != nil {
		t.Fatal(err)
	}

	wait := make(chan Event)
	go func() {
		for msg := range es.Events {
			for _, event := range msg {
				t.Logf("Event: %#v", event)
				wait <- event
				es.Stop()
				return
			}
		}
	}()

	err = os.WriteFile(filepath.Join(path, "example.txt"), []byte("example"), 0700)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-wait:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestIssue48(t *testing.T) {
	// FSEvents fails to start when watching >4096 paths
	// This test validates that limit and checks that the error is propagated

	path, err := os.MkdirTemp("", "fsmanyfiles")
	if err != nil {
		t.Fatal(err)
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(path)

	// TODO: using this value fails to start
	// dev, err := DeviceForPath(path)
	// if err != nil {
	// 	t.Fatal(err)
	// }

	var filenames []string
	for i := 0; i < 4096; i++ {
		newFilename := filepath.Join(path, fmt.Sprint("test", i))
		err = os.WriteFile(newFilename, []byte("test"), 0700)
		if err != nil {
			t.Fatal(err)
		}
		filenames = append(filenames, newFilename)
	}

	es := &EventStream{
		Paths:   filenames,
		Latency: 500 * time.Millisecond,
		Device:  0, //dev,
		Flags:   FileEvents,
	}

	err = es.Start()
	if err != nil {
		t.Fatal(err)
	}

	wait := make(chan Event)
	go func() {
		for msg := range es.Events {
			for _, event := range msg {
				t.Logf("Event: %#v", event)
				wait <- event
				es.Stop()
				return
			}
		}
	}()

	// write some new contents to test42 in the watchlist
	err = os.WriteFile(filenames[42], []byte("special"), 0700)
	if err != nil {
		t.Fatal(err)
	}

	// should be reported as expected
	<-wait

	/////
	// create one more file that puts it over the edge
	newFilename := filepath.Join(path, fmt.Sprint("test", 4096))
	err = os.WriteFile(newFilename, []byte("test"), 0700)
	if err != nil {
		t.Fatal(err)
	}
	filenames = append(filenames, newFilename)

	// create an all-new instances to avoid problems
	es2 := &EventStream{
		Paths:   filenames,
		Latency: 500 * time.Millisecond,
		Device:  0, //dev,
		Flags:   FileEvents,
	}

	err = es2.Start()
	if err == nil {
		es2.Stop()
		t.Fatal("eventstream error was not detected on >4096 files in watchlist")
	}
}

func TestRegistry(t *testing.T) {
	if registry.m == nil {
		t.Fatal("registry not initialized at start")
	}

	es := &EventStream{}
	i := registry.Add(es)

	if registry.Get(i) == nil {
		t.Fatal("failed to retrieve es from registry")
	}

	if es != registry.Get(i) {
		t.Errorf("eventstream did not match what was found in the registry")
	}

	registry.Delete(i)
	if registry.Get(i) != nil {
		t.Error("failed to delete registry")
	}
}

func TestMany(t *testing.T) {
	tmp := t.TempDir()

	path, err := filepath.EvalSymlinks(tmp)
	if err != nil {
		t.Fatal(err)
	}

	dev, err := DeviceForPath(path)
	if err != nil {
		t.Fatal(err)
	}

	es := &EventStream{
		Paths:   []string{path},
		Latency: 0,
		Device:  dev,
		Flags:   FileEvents | NoDefer,
	}

	err = es.Start()
	if err != nil {
		t.Fatal(err)
	}

	//   300 = 100 files in root folder, 3 events per
	// + 300 = 100 files in subfolders, 3 events per
	// + 200 = 100 subfolders, 2 events per
	// + 1 root folder
	expectedEventCount := 300 + 300 + 200 + 1

	var doneWg sync.WaitGroup
	doneWg.Add(expectedEventCount)

	events := make(map[string]EventFlags, expectedEventCount)
	go func() {
		for msg := range es.Events {
			for _, event := range msg {
				if _, ok := events[event.Path]; !ok {
					events[event.Path] = event.Flags
				} else {
					events[event.Path] = events[event.Path].set(event.Flags)
				}

				doneWg.Done()
			}
		}
	}()

	const data = "data"

	var wg sync.WaitGroup
	wg.Add(200)

	for i := 0; i < 100; i++ {
		i := i

		go func() {
			defer wg.Done()
			echoAppend(t, data, fmt.Sprintf("%s/file-%d", path, i))
		}()

		go func() {
			defer wg.Done()
			mkdir(t, fmt.Sprintf("%s/dir-%d", path, i))
		}()
	}
	wg.Wait()

	wg.Add(100)
	for i := 0; i < 100; i++ {
		i := i

		go func() {
			defer wg.Done()
			echoAppend(t, data, fmt.Sprintf("%s/dir-%d/file-%d", path, i, i))
		}()
	}
	wg.Wait()

	wg.Add(200)
	for i := 0; i < 100; i++ {
		i := i
		go func() {
			defer wg.Done()
			rm(t, fmt.Sprintf("%s/file-%d", tmp, i))
		}()
		go func() {
			defer wg.Done()
			rmAll(t, fmt.Sprintf("%s/dir-%d", tmp, i))
		}()
	}
	wg.Wait()

	doneWg.Wait()

	const fileExpectedFlags = ItemIsFile | ItemCreated | ItemModified | ItemRemoved
	const dirExpectedFlags = ItemIsDir | ItemCreated | ItemRemoved

	for p, flags := range events {
		if p == strings.TrimPrefix(path, "/") {
			continue
		}

		switch {
		case flags.hasFlag(ItemIsFile):
			if flags.String() != fileExpectedFlags.String() {
				t.Fatalf(
					"file flags for path '%s' did not match expected '%s' found '%s'",
					p,
					fileExpectedFlags.String(),
					flags.String(),
				)
			}
		case flags.hasFlag(ItemIsDir):
			if flags.String() != dirExpectedFlags.String() {
				t.Fatalf(
					"file flags for path '%s' did not match expected '%s' found '%s'",
					p,
					dirExpectedFlags.String(),
					flags.String(),
				)
			}
		default:
			t.Fatal("unrecognized flag")
		}
	}
}
