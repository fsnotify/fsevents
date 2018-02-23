// +build darwin

package fsevents

import (
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

	es.Start()

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
