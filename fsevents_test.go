//go:build darwin
// +build darwin

package fsevents

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBasicExample(t *testing.T) {
	path, err := ioutil.TempDir("", "fsexample")
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

	err = ioutil.WriteFile(filepath.Join(path, "example.txt"), []byte("example"), 0700)
	if err != nil {
		t.Fatal(err)
	}

	<-wait
}

func TestIssue48(t *testing.T) {
	// FSEvents fails to start when watching >4096 paths
	// This test validates that limit and checks that the error is propagated

	path, err := ioutil.TempDir("", "fsmanyfiles")
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
		err = ioutil.WriteFile(newFilename, []byte("test"), 0700)
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
	err = ioutil.WriteFile(filenames[42], []byte("special"), 0700)
	if err != nil {
		t.Fatal(err)
	}

	// should be reported as expected
	<-wait

	/////
	// create one more file that puts it over the edge
	newFilename := filepath.Join(path, fmt.Sprint("test", 4096))
	err = ioutil.WriteFile(newFilename, []byte("test"), 0700)
	if err != nil {
		t.Fatal(err)
	}
	filenames = append(filenames, newFilename)

	// create an all-new instances to avoid problems
	es = &EventStream{
		Paths:   filenames,
		Latency: 500 * time.Millisecond,
		Device:  0, //dev,
		Flags:   FileEvents,
	}

	err = es.Start()
	if err == nil {
		es.Stop()
		t.Fatal("eventstream error was not detected on >4096 files in watchlist")
	}
}
