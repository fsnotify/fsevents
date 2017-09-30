// +build darwin

package fsevents

import (
	"io/ioutil"
	"os"
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
}
