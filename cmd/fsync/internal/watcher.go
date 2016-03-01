package internal

import "gopkg.in/fsnotify.v1"

var (
	WatchExit chan bool
)

func init() {
	WatchExit = make(chan bool)
}

// Watch watches the given path for changes using fsnotify and will upload
// files as needed
func Watch(path string) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		Log.Fatalf("Could not initialize fsnotify watcher: %v\n", err)
	}

	watcher.Add(path)

	for {
		select {
		case event := <-watcher.Events:
			Log.Debugln("Event:", event)
			if event.Op&fsnotify.Write == fsnotify.Write {
				Log.Debugln("Write event received -- uploading file")
				ModifiedFiles <- event.Name
			} else if event.Op&fsnotify.Remove == fsnotify.Remove {
				Log.Debugln("Remove event received -- removing file")
				DeletedFiles <- event.Name
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
}
