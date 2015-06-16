// This is a wrapper around a recursive watcher
// It is needed to filter through the discrepancies in notifications
// accross platforms, or simply strange notifications.
// i.e.: double create on windows (https://github.com/howeyc/fsnotify/issues/106)
package watch

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
  
  "thierry/sync/md5"
)

type FileEvent struct {
  EventType Event
  FileType  string
}

type GlobalWatcher struct {
  Events chan FileEvent
  Errors chan error
  Ignore string
  Watcher *RecursiveWatcher
  FileHash map[string]string
}

func NewWatcher(rootdir string, except string) (*GlobalWatcher, error) {
  watcher, err := NewRecursiveWatcher(rootdir, except)
  if err != nil {
		log.Println("Watcher create error : ", err)
    return nil, err
	}
	defer watcher.Close()
	_done := make(chan bool)
  
	globalwatcher := &GlobalWatcher{Watcher: watcher}
	globalwatcher.Events = make(chan Event)
	globalwatcher.Errors = make(chan error)
	globalwatcher.Ignore = except
  
  return globalwatcher
}

func (w GlobalWatcher) start() {
	go func() {
		for {
			select {
			case event := <-w.watcher.Events:
				if ShouldIgnoreFile(filepath.Base(event.Name), w.Ignore) {
					continue
				}
				switch {
				// create a file/directory
				case event.Op&fsnotify.Create == fsnotify.Create:
					fi, err := os.Stat(event.Name)
					if err != nil {
						fmt.Println(err)
						continue
					} else if fi.IsDir() {
						fmt.Println("Detected new directory", event.Name)
						if !ShouldIgnoreFile(filepath.Base(event.Name), w.Ignore) {
							fmt.Println("Monitoring new folder...")
							w.watcher.AddFolder(event.Name)
              w.Events <- FileEvent{EventType: event, FileType: "folder"}
						}
					} else {
            // Here double check if file has changed before sending event
            if w.hasFileChanged(event.Name) {
              w.Events <- FileEvent{EventType: event, FileType: "file"}
            } else {
							fmt.Println("File created notification has not changed", event.Name)
            }
					}

				case event.Op&fsnotify.Write == fsnotify.Write:
					// modified a file, assuming that you don't modify folders
					fmt.Println("Detected file modification %s", event.Name)
					// Don't handle folder change, since they receive notification
					// when a file they contain is changed
					fi, err := os.Stat(event.Name)
					if err != nil {
						fmt.Println(err)
						continue
					}
					if fi.Mode().IsRegular() {
						log.Println("Modified file: ", event.Name)
            // Here double check if file has changed before sending event
            if w.hasFileChanged(event.Name) {
              w.Events <- FileEvent{EventType: event, FileType: "file"}
            } else {
							fmt.Println("File modified notification has not changed", event.Name)
            }
					}
				case event.Op&fsnotify.Remove == fsnotify.Remove:
					log.Println("Removed: ", event.Name)
          w.Events <- FileEvent{EventType: event}
          
				case event.Op&fsnotify.Rename == fsnotify.Rename:
					log.Println("Renamed file: ", event.Name)
					// The following is to handle an issue in fsnotify
					// On rename, fsnotify sends three events on linux: RENAME(old), CREATE(new), RENAME(new)
					// fsnotify sends two events on windows: RENAME(old), CREATE(new)
					_, err := os.Stat(event.Name)
					if err != nil {
						// Rename talks about a file/folder now gone, send a remove request to server
						log.Println("Rename leading to delete", event.Name)
						connsender := connectToServer(cafile, server)
						go sendClientDelete(connsender, event.Name, listFileInProcess)
					} else {
						// Rename talks about a file/folder already existing, skip it (do nothing)
					}
				case event.Op&fsnotify.Chmod == fsnotify.Chmod:
					log.Println("File changed permission: ", event.Name)
				}

			case err := <-w.watcher.Errors:
				log.Println("w watching error : ", err)
				_done <- true
				done <- true
			}
		}

	}()
}