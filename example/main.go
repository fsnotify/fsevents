// +build darwin

package main

import (
	"bufio"
	"log"
	"os"
	"time"

	"github.com/go-fsnotify/fsevents"
)

func main() {
	//	dev := fsevents.DeviceForPath("/tmp")
	// log.Print(dev)
	es := &fsevents.EventStream{
		Paths:   []string{"/tmp"},
		Latency: 500 * time.Millisecond,
		//		Device:  dev,
		Flags: fsevents.FileEvents | fsevents.WatchRoot}
	es.Start()

	go func() {
		for msg := range es.Events {
			for _, event := range msg {
				logEvent(event)
			}
		}
	}()
	log.Print("Started")

	// press enter to continue
	in := bufio.NewReader(os.Stdin)
	in.ReadString('\n')
	es.Stop()
}

var noteDescription = map[fsevents.EventFlags]string{
	fsevents.MustScanSubDirs: "MustScanSubdirs",
	fsevents.UserDropped:     "UserDropped",
	fsevents.KernelDropped:   "KernelDropped",
	fsevents.EventIdsWrapped: "EventIdsWrapped",
	fsevents.HistoryDone:     "HistoryDone",
	fsevents.RootChanged:     "RootChanged",
	fsevents.Mount:           "Mount",
	fsevents.Unmount:         "Unmount",

	fsevents.ItemCreated:       "Created",
	fsevents.ItemRemoved:       "Removed",
	fsevents.ItemInodeMetaMod:  "InodeMetaMod",
	fsevents.ItemRenamed:       "Renamed",
	fsevents.ItemModified:      "Modified",
	fsevents.ItemFinderInfoMod: "FinderInfoMod",
	fsevents.ItemChangeOwner:   "ChangeOwner",
	fsevents.ItemXattrMod:      "XAttrMod",
	fsevents.ItemIsFile:        "IsFile",
	fsevents.ItemIsDir:         "IsDir",
	fsevents.ItemIsSymlink:     "IsSymLink",
}

func logEvent(event fsevents.Event) {
	note := ""
	for bit, description := range noteDescription {
		if event.Flags&bit == bit {
			note += description + " "
		}
	}
	log.Printf("EventID: %d Path: %s Flags: %s", event.Id, event.Path, note)
}
