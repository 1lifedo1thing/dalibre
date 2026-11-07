// Package convert describes conversions between e-book formats and finds
// the engine that will run them. Version 1 has the model and the UI;
// running conversions comes next (calibre's ebook-convert behind Engine).
package convert

import (
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// Formats are the outputs Dalibre offers, in the order the UI shows them.
var Formats = []Format{
	{"EPUB", "Reflowable", "Best for most e-readers, Apple Books and Kobo."},
	{"PDF", "Fixed layout", "For print, tablets and annotation."},
	{"MOBI", "Kindle (legacy)", "Older Kindle devices and apps."},
}

// Format is an output format and how the UI describes it.
type Format struct {
	Name, Title, Description string
}

// Profiles are calibre's output profiles the UI offers.
var Profiles = []Profile{
	{"default", "Default"},
	{"tablet", "Tablet"},
	{"kindle_pw3", "Kindle Paperwhite"},
	{"kindle_oasis", "Kindle Oasis"},
	{"kindle_scribe", "Kindle Scribe"},
	{"kobo", "Kobo"},
	{"generic_eink", "Generic e-ink"},
}

// Profile is one of calibre's output profiles.
type Profile struct{ ID, Title string }

// Options are what the convert dialog sets.
type Options struct {
	Output       string // "EPUB", "PDF" or "MOBI"
	Profile      string
	EmbedFonts   bool
	KeepTOC      bool
	KeepOriginal bool
	PaperSize    string // PDF only, as "a5"
}

// DefaultOptions are the dialog's starting values.
func DefaultOptions() Options {
	return Options{Output: "EPUB", Profile: "default", EmbedFonts: true, KeepTOC: true, KeepOriginal: true, PaperSize: "a5"}
}

// Engine is a program that converts.
type Engine struct {
	Name string
	Path string
}

// FindEngine looks for calibre's ebook-convert: on the PATH, then where
// calibre installs itself.
func FindEngine() (Engine, bool) {
	if p, err := exec.LookPath("ebook-convert"); err == nil {
		return Engine{"calibre ebook-convert", p}, true
	}
	var candidates []string
	switch runtime.GOOS {
	case "windows":
		candidates = []string{`C:\Program Files\Calibre2\ebook-convert.exe`, `C:\Program Files (x86)\Calibre2\ebook-convert.exe`}
	case "darwin":
		candidates = []string{"/Applications/calibre.app/Contents/MacOS/ebook-convert"}
	default:
		candidates = []string{"/usr/bin/ebook-convert", "/opt/calibre/ebook-convert"}
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return Engine{"calibre ebook-convert", p}, true
		}
	}
	return Engine{}, false
}

// ErrNotImplemented is what Run returns until the engine is wired in.
var ErrNotImplemented = errors.New("conversion isn’t available yet in this version of Dalibre")

// Status is where a job is.
type Status int

const (
	Queued Status = iota
	Running
	Done
	Failed
)

// Job is the conversion of one book.
type Job struct {
	BookID   int64
	Title    string
	From, To string
	Status   Status
	Progress float64 // 0–1 while running
	Message  string
}

// Label is the job's conversion, as "EPUB → MOBI".
func (j *Job) Label() string { return strings.ToUpper(j.From) + " → " + strings.ToUpper(j.To) }

// Run converts the job's book. It is not implemented yet.
func Run(j *Job, opts Options) error { return ErrNotImplemented }

// SourceFormat picks the best input among a book's formats for an output.
func SourceFormat(formats []string, output string) string {
	for _, f := range []string{"EPUB", "AZW3", "MOBI", "DOCX", "FB2", "PDF", "TXT"} {
		if f == output {
			continue
		}
		for _, have := range formats {
			if strings.EqualFold(have, f) {
				return f
			}
		}
	}
	for _, have := range formats {
		if !strings.EqualFold(have, output) {
			return strings.ToUpper(have)
		}
	}
	return ""
}
