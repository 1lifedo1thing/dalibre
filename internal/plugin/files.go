package plugin

import (
	"io/fs"
	"path/filepath"
	"syscall"
)

// DataMount is where a plugin's script sees its data folder through
// node:fs.
const DataMount = "/data"

// MaxData is how much a plugin may keep in its data folder.
const MaxData = 64 << 20

// DataDir is the data folder of plugin id whose plugins folder is root:
// a hidden folder beside the plugins, which Discover skips and Uninstall
// removes with the plugin.
func DataDir(root, id string) string {
	return filepath.Join(root, ".data", id)
}

// allowFile approves each access of node:fs to the data folder: writes
// only while the folder holds less than MaxData.
func (p *Plugin) allowFile(op, _ string) error {
	if op != "write" {
		return nil
	}
	if size := folderSize(p.dataDir); size >= MaxData {
		p.logf("[%s] its data folder is full (%d MB)", p.Manifest.ID, MaxData>>20)
		return syscall.ENOSPC
	}
	return nil
}

// folderSize adds up the sizes of the files in dir.
func folderSize(dir string) int64 {
	var n int64
	filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			if info, err := d.Info(); err == nil {
				n += info.Size()
			}
		}
		return nil
	})
	return n
}
