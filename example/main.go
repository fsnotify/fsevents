// +build darwin

package main

import (
	"bufio"
	"log"
	"os"

	"github.com/go-fsnotify/fsevents"
)

func main() {
	es := fsevents.CreateEventStream([]string{"/tmp"}, 0)

	go func() {
		for msg := range es.C {
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

var noteDescription = map[uint32]string{
	fsevents.EventFlagMustScanSubDirs: "MustScanSubdirs",
	fsevents.EventFlagUserDropped:     "UserDropped",
	fsevents.EventFlagKernelDropped:   "KernelDropped",
	fsevents.EventFlagEventIdsWrapped: "EventIdsWrapped",
	fsevents.EventFlagHistoryDone:     "HistoryDone",
	fsevents.EventFlagRootChanged:     "RootChanged",
	fsevents.EventFlagMount:           "Mount",
	fsevents.EventFlagUnMount:         "Unmount",

	fsevents.EventFlagItemCreated:       "Created",
	fsevents.EventFlagItemRemoved:       "Removed",
	fsevents.EventFlagItemInodeMetaMod:  "InodeMetaMod",
	fsevents.EventFlagItemRenamed:       "Renamed",
	fsevents.EventFlagItemModified:      "Modified",
	fsevents.EventFlagItemFinderInfoMod: "FinderInfoMod",
	fsevents.EventFlagItemChangeOwner:   "ChangeOwner",
	fsevents.EventFlagItemXattrMod:      "XAttrMod",
	fsevents.EventFlagItemIsFile:        "IsFile",
	fsevents.EventFlagItemIsDir:         "IsDir",
	fsevents.EventFlagItemIsSymlink:     "IsSymLink",
}

func logEvent(event fsevents.FSEvent) {
	note := ""
	for bit, description := range noteDescription {
		if event.Flags&bit == bit {
			note += description + " "
		}
	}
	log.Printf("EventID: %d Path: %s Flags: %s", event.Id, event.Path, note)
}
