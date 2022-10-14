# FSEvents bindings for Go (macOS)

[![GoDoc](https://godoc.org/github.com/fsnotify/fsevents?status.svg)](https://godoc.org/github.com/fsnotify/fsevents) [![Reviewed by Hound](https://img.shields.io/badge/Reviewed_by-Hound-8E64B0.svg)](https://houndci.com)

[FSEvents](https://developer.apple.com/library/mac/documentation/Darwin/Reference/FSEvents_Ref/) allows an application to monitor a whole file system or portion of it. FSEvents is only available on macOS.

**Warning:** This API should be considered unstable.

## Caveats

Known caveats of the macOS FSEvents API which this package uses under the hood:

 - FSEvents returns events for the named path only, so unless you want to follow updates to a symlink itself (unlikely), you should use `filepath.EvalSymlinks` to get the target path to watch.
 - There is an internal macOS limitation of 4096 watched paths. Watching more paths will result in an error calling `Start()`. Note that FSEvents is intended to be a recursive watcher by design, it is actually more efficient to watch the containing path than each file in a large directory.

## Contributing

Request features and report bugs using the [GitHub Issue Tracker](https://github.com/fsnotify/fsevents/issues).

fsevents carries the same [LICENSE](https://github.com/fsnotify/fsevents/blob/master/LICENSE) as Go. Contributors retain their copyright, so you need to fill out a short form before we can accept your contribution: [Google Individual Contributor License Agreement](https://developers.google.com/open-source/cla/individual).
