package app

import (
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// defaultWatchEvery is how often linked plugins' folders are looked at, to
// reload them when their files change.
const defaultWatchEvery = time.Second

// folderWatch tells when the files in some folders change, by their
// names, sizes and times: plugins are small, so walking them is cheap.
type folderWatch struct {
	dirs []string
	last string // the fingerprint seen last
}

func newFolderWatch(dirs []string) *folderWatch {
	if len(dirs) == 0 {
		return nil
	}
	w := &folderWatch{dirs: slices.Clone(dirs)}
	w.last = w.fingerprint()
	return w
}

// fingerprint sums up the folders' files as packing sees them: hidden
// files and node_modules left out.
func (w *folderWatch) fingerprint() string {
	var b strings.Builder
	for _, dir := range w.dirs {
		files := 0
		filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				b.WriteString("!" + p + "\n")
				return nil
			}
			if p != dir && (strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules") {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				return nil
			}
			if files++; files > 2000 {
				return filepath.SkipAll
			}
			if info, err := d.Info(); err == nil {
				b.WriteString(p)
				b.WriteString(info.ModTime().Format(time.RFC3339Nano))
				b.WriteString(" " + strconv.FormatInt(info.Size(), 10) + "\n")
			}
			return nil
		})
	}
	return b.String()
}

// changed reports whether the files changed since it last said so, once
// they have stayed the same for a look: an editor saving several files,
// or a build writing one, is reloaded once, when it is done.
func (w *folderWatch) changed() (bool, func() bool) {
	f := w.fingerprint()
	if f == w.last {
		return false, nil
	}
	return true, func() bool {
		if g := w.fingerprint(); g != f {
			f = g
			return false
		}
		w.last = f
		return true
	}
}

// watchLinked looks at linked plugins' folders until the plugins are
// loaded again, and reloads them when their files change.
func (a *App) watchLinked(w *folderWatch) {
	gen := a.linkGen.Add(1)
	if w == nil || a.watchEvery <= 0 {
		return
	}
	every := a.watchEvery
	go func() {
		t := time.NewTicker(every)
		defer t.Stop()
		var settled func() bool
		for range t.C {
			if a.linkGen.Load() != gen {
				return
			}
			if settled == nil {
				if ch, s := w.changed(); ch {
					settled = s
				}
				continue
			}
			if !settled() {
				continue
			}
			a.post(func() {
				if a.linkGen.Load() == gen {
					a.loadPlugins()
				}
			})
			return
		}
	}()
}
