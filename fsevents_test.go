// +build darwin

package fsevents

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newEventStream(t *testing.T, path string, useDev bool) *EventStream {
	es := &EventStream{
		Paths:   []string{path},
		Latency: 500 * time.Millisecond,
		Flags:   FileEvents,
	}

	if useDev {
		dev, err := DeviceForPath(path)
		if err != nil {
			t.Fatal(err)
		}

		es.Device = dev
	}

	return es
}

func processEvents(t *testing.T, es *EventStream, wait chan Event) {
	for msg := range es.Events {
		for _, event := range msg {
			t.Logf("Event: %#v", event)
			wait <- event
			es.Stop()
			return
		}
	}
}

func TestBasicExample(t *testing.T) {
	path, err := ioutil.TempDir("", "fsexample")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(path)

	es := newEventStream(t, path, true)

	es.Start()

	wait := make(chan Event)
	go processEvents(t, es, wait)

	err = ioutil.WriteFile(filepath.Join(path, "example.txt"), []byte("example"), 0600)
	if err != nil {
		t.Fatal(err)
	}

	<-wait
}

func TestNoDevice(t *testing.T) {
	path, err := ioutil.TempDir("", "fsexample")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(path)

	es := newEventStream(t, path, false)

	es.Start()

	wait := make(chan Event)
	go processEvents(t, es, wait)

	err = ioutil.WriteFile(filepath.Join(path, "example.txt"), []byte("example"), 0600)
	if err != nil {
		t.Fatal(err)
	}

	<-wait
}
