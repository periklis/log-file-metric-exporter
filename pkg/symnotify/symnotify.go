// package symnotify provides a file system watcher that notifies events for symlink targets.
//
package symnotify

import (
	"github.com/ViaQ/logerr/log"
	"github.com/fsnotify/fsnotify"
	"io"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"time"
)

type Event = fsnotify.Event
type Op = fsnotify.Op

const (
	Create Op = fsnotify.Create
	Write     = fsnotify.Write
	Remove    = fsnotify.Remove
	Rename    = fsnotify.Rename
	Chmod     = fsnotify.Chmod
)

// Watcher is like fsnotify.Watcher but also notifies on changes to symlink targets
type Watcher struct {
	watcher *fsnotify.Watcher
}

func NewWatcher() (*Watcher, error) {
	w, err := fsnotify.NewWatcher()
	return &Watcher{watcher: w}, err
}

// Event returns the next event.
func (w *Watcher) Event() (e Event, err error) {
	return w.EventTimeout(time.Duration(math.MaxInt64))
}

// EventTimeout returns the next event or os.ErrDeadlineExceeded if timeout is exceeded.
func (w *Watcher) EventTimeout(timeout time.Duration) (e Event, err error) {
	var ok bool
	select {
	case e, ok = <-w.watcher.Events:
	case err, ok = <-w.watcher.Errors:
	case <-time.After(timeout):
		return Event{}, os.ErrDeadlineExceeded
	}
	switch {
	case !ok:
		return Event{}, io.EOF
	case e.Op == Create:
		log.V(2).Info("Create Event Detected for file..", "e.Name", e.Name)
		if info, err := os.Lstat(e.Name); err == nil {
			if isSymlink(info) {
				_ = w.watcher.Add(e.Name)
			}
		}
	case e.Op == Remove:
		log.V(2).Info("Remove Event Detected for file..", "e.Name", e.Name)
	case e.Op == Chmod || e.Op == Rename:
		log.V(2).Info("Chmod or Rename Event Detected for file..", "e.Name", e.Name)
		if info, err := os.Lstat(e.Name); err == nil {
			if isSymlink(info) {
				// Symlink target may have changed.
				_ = w.watcher.Remove(e.Name)
				_ = w.watcher.Add(e.Name)
			}
		}
	}
	return e, err
}

// Add dir,dir/files* to the watcher
func (w *Watcher) Add(name string) error {
	if err := w.watcher.Add(name); err != nil {
		return err
	}

	// Scan directories for existing symlinks, we wont' get a Create for those.
	if infos, err := ioutil.ReadDir(name); err == nil {
		for _, info := range infos {
			if isSymlink(info) {
				log.V(3).Info("Adding file to watcher ...", "filename", filepath.Join(name, info.Name()))
				err := w.watcher.Add(filepath.Join(name, info.Name()))
				log.V(3).Info("err return by watcher.Add call ...", "err", err)
			}
		}
	}
	return nil
}

// Remove name from watcher
func (w *Watcher) Remove(name string) error {
	//delete(w.added, name)
	return w.watcher.Remove(name)
}

// Close watcher
func (w *Watcher) Close() error { return w.watcher.Close() }

func isSymlink(info os.FileInfo) bool {
	return (info.Mode() & os.ModeSymlink) == os.ModeSymlink
}
