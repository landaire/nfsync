package internal

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/landaire/recwatch"
	"gopkg.in/fsnotify.v1"
)

var (
	WatchExit chan bool
	watchRoot string
)

func init() {
	WatchExit = make(chan bool)
}

// Watch watches the given path for changes using fsnotify and will upload
// files as needed
func Watch(path string) {
	Log.Debugf("Adding %s to watcher\n", path)
	watcher, err := recwatch.NewRecursiveWatcher(path)
	if err != nil {
		Log.Fatalf("Could not initialize fsnotify watcher: %v\n", err)
	}

	watchRoot = path

	for {
		select {
		case event := <-watcher.Events:
			Log.Debugln("Event:", event)
			if strings.TrimSpace(event.Name) == "" {
				Log.Debugln("Got an empty string... not touching that")
				continue
			}

			path, err := filePathFromEvent(&event)
			if err != nil {
				Log.Errorln(err)
				continue
			}

			if event.Op&fsnotify.Write == fsnotify.Write {
				Log.Debugln("Write event received:", event.Name)
				ModifiedFiles <- path
			} else if event.Op&fsnotify.Remove == fsnotify.Remove {
				Log.Debugln("Remove event received:", event.Name)
				DeletedFiles <- path
			} else if event.Op&fsnotify.Create == fsnotify.Create {
				Log.Debugln("Create event received:", event.Name)
				ModifiedFiles <- path

				stat, err := os.Stat(path)
				if err != nil {
					Log.Errorln(err)
					continue
				}

				if stat.IsDir() {
					if err := watcher.Add(path); err != nil {
						Log.Debugln("Couldn't watch folder:", err)
					}
				}
			} else if event.Op&fsnotify.Rename == fsnotify.Rename {
				Log.Debugln("Rename event received:", event.Name)
				DeletedFiles <- path
			}
		case <-WatchExit:
			Log.Debugln("WatchExit signal received -- shutting down watcher")
			goto _cleanup
		}
	}

_cleanup:
	if err := watcher.Close(); err != nil {
		Log.Errorln("Error shutting down watcher:", err)
	}

	Log.Debugln("Exiting Watcher")
	WatchExit <- true
}

func pathRelativeToWatchRoot(path string) (string, error) {
	return filepath.Rel(watchRoot, path)
}

func filePathFromEvent(event *fsnotify.Event) (path string, err error) {
	defer func() { path = filepath.Clean(path) }()

	if filepath.IsAbs(event.Name) {
		path = event.Name
		return
	}

	path, err = filepath.Abs(event.Name)
	return
}
