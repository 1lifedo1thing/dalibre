//go:build !darwin

package app

import "github.com/egoist/mygo"

// shareSupported reports whether shareFiles shows the system's share sheet;
// only macOS has one wired up.
const shareSupported = false

// shareFiles shows the system's share sheet for the files at paths.
var shareFiles = func(*mygo.Window, []string) {}
