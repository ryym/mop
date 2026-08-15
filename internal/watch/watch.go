// Package watch wraps fsnotify for the way editors actually save files.
//
// Two things matter here:
//
//   - The parent directory is watched, not the file. Many editors save by
//     writing a temp file and renaming it over the original, which changes the
//     inode and silently drops a watch registered on the file itself.
//   - Events are debounced. One save produces several events (CREATE, RENAME,
//     CHMOD ...), and an in-place write can be observed halfway through.
//     Waiting for the file to go quiet avoids both.
package watch

import (
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// DefaultDebounce is how long a watched file must stay quiet before the
// change is reported.
const DefaultDebounce = 50 * time.Millisecond

// Watcher reports changes to individually registered files.
type Watcher struct {
	fsw      *fsnotify.Watcher
	debounce time.Duration
	onChange func(path string)

	mu    sync.Mutex
	files map[string]bool        // absolute file path -> watched
	dirs  map[string]int         // parent directory -> number of files
	timer map[string]*time.Timer // pending debounce per file
	done  chan struct{}
	once  sync.Once
}

// New starts a watcher. onChange is called from a background goroutine with
// the absolute path of the file that changed.
func New(onChange func(path string)) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	w := &Watcher{
		fsw:      fsw,
		debounce: DefaultDebounce,
		onChange: onChange,
		files:    map[string]bool{},
		dirs:     map[string]int{},
		timer:    map[string]*time.Timer{},
		done:     make(chan struct{}),
	}
	go w.loop()
	return w, nil
}

// Add starts watching an absolute file path. Adding the same path twice is a
// no-op.
func (w *Watcher) Add(path string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.files[path] {
		return nil
	}
	dir := filepath.Dir(path)
	if w.dirs[dir] == 0 {
		if err := w.fsw.Add(dir); err != nil {
			return err
		}
	}
	w.files[path] = true
	w.dirs[dir]++
	return nil
}

// Remove stops watching a file. The parent directory stays watched while any
// other registered file lives in it.
func (w *Watcher) Remove(path string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.files[path] {
		return nil
	}
	delete(w.files, path)
	if t := w.timer[path]; t != nil {
		t.Stop()
		delete(w.timer, path)
	}
	dir := filepath.Dir(path)
	w.dirs[dir]--
	if w.dirs[dir] <= 0 {
		delete(w.dirs, dir)
		return w.fsw.Remove(dir)
	}
	return nil
}

// Close stops the watcher.
func (w *Watcher) Close() error {
	w.once.Do(func() { close(w.done) })
	return w.fsw.Close()
}

func (w *Watcher) loop() {
	for {
		select {
		case <-w.done:
			return
		case ev, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			w.schedule(filepath.Clean(ev.Name))
		case _, ok := <-w.fsw.Errors:
			// Errors are not fatal for a preview tool; keep watching.
			if !ok {
				return
			}
		}
	}
}

func (w *Watcher) schedule(path string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.files[path] {
		return
	}
	if t := w.timer[path]; t != nil {
		t.Stop()
	}
	w.timer[path] = time.AfterFunc(w.debounce, func() {
		w.mu.Lock()
		delete(w.timer, path)
		watched := w.files[path]
		w.mu.Unlock()
		if watched {
			w.onChange(path)
		}
	})
}
