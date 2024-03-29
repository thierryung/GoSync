// From https://github.com/nathany/looper/blob/master/watch.go
package watch

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/fsnotify.v1"
)

type RecursiveWatcher struct {
	*fsnotify.Watcher
	Files   chan string
	Folders chan string
}

func NewRecursiveWatcher(path string, except string) (*RecursiveWatcher, error) {
	folders := Subfolders(path, except)
	if len(folders) == 0 {
		return nil, errors.New("No folders to watch.")
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	rw := &RecursiveWatcher{Watcher: watcher}

	rw.Files = make(chan string, 10)
	rw.Folders = make(chan string, len(folders))

	for _, folder := range folders {
		rw.AddFolder(folder)
	}
	return rw, nil
}

func (watcher *RecursiveWatcher) AddFolder(folder string) {
	err := watcher.Add(folder)
	if err != nil {
		log.Println("Error watching: ", folder, err)
	}
}

// Subfolders returns a slice of subfolders (recursive), including the folder provided.
func Subfolders(path string, except string) (paths []string) {
	filepath.Walk(path, func(newPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

    // log.Println(info.Name())
    // log.Println(newPath)

		if info.IsDir() {
			name := info.Name()
			// skip folders that begin with a dot
      // TODO: test if name != "." && name != ".." is needed
			if ShouldIgnoreFile(name, except) && name != "." && name != ".." {
				return filepath.SkipDir
			}
			paths = append(paths, newPath)
		}
		return nil
	})
	return paths
}

// shouldIgnoreFile determines if a file should be ignored.
// File names that begin with "." or "_" are ignored by the go tool.
func ShouldIgnoreFile(name string, except string) bool {
	return strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || name == except
}
