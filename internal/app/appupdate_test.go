package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type fakeUpdater struct {
	release *appRelease
	err     error
	checks  int
	beta    bool
}

func (f *fakeUpdater) Check(_ context.Context, beta bool) (*appRelease, error) {
	f.checks++
	f.beta = beta
	return f.release, f.err
}

func newRelease(installErr error) *appRelease {
	return &appRelease{
		Version: "0.2.0",
		Notes:   "## 0.2.0\n\n- Series shelves: books in a series stand together\n- **Faster** import of large folders\n",
		Date:    time.Now().Add(-48 * time.Hour),
		Install: func(_ context.Context, progress func(done, total int64)) error {
			progress(14_600_000, 36_500_000)
			return installErr
		},
	}
}

func TestAppUpdateCheckInSettings(t *testing.T) {
	a := newTestApp(t)
	a.au.version = "0.1.0"
	f := &fakeUpdater{}
	a.au.updater = f
	tt := ui.NewTester(a.View, 1440, 900)

	a.openSettings(sectionGeneral)
	tt.Frame()
	for _, s := range []string{"Dalibre 0.1.0", "Not checked yet", "Check for updates automatically", "Include beta versions"} {
		if !tt.HasText(s) {
			t.Fatalf("misses %q: %q", s, tt.Texts())
		}
	}
	if err := tt.Click("Check for updates"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if f.checks != 1 || !tt.HasText("Up to date · checked just now") {
		t.Fatalf("up to date: %d checks, %q", f.checks, tt.Texts())
	}
	shot(t, tt, "app-update-current")

	f.err = errors.New("offline")
	a.checkAppUpdate(true)
	tt.Frame()
	if !tt.HasText("Couldn’t check for updates") || !tt.HasText("Try again") {
		t.Fatalf("failed: %q", tt.Texts())
	}
	f.err = mygo.ErrUpdatesDisabled
	a.checkAppUpdate(true)
	tt.Frame()
	if !tt.HasText("This copy of Dalibre can’t update itself") {
		t.Fatalf("disabled: %q", tt.Texts())
	}

	f.err, f.release = nil, newRelease(nil)
	relaunched := 0
	a.au.relaunch = func() { relaunched++ }
	a.checkAppUpdate(true)
	tt.Frame()
	if a.au.dialog || !tt.HasText("Dalibre 0.2.0 is available") || !tt.HasText("Install and restart") {
		t.Fatalf("available: dialog %v, %q", a.au.dialog, tt.Texts())
	}
	shot(t, tt, "app-update-available")
	if err := tt.Click("Install and restart"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.au.status != auReady || relaunched != 1 || !tt.HasText("Dalibre 0.2.0 is ready") || !tt.HasText("Restart now") {
		t.Fatalf("ready: status %v, relaunched %d, %q", a.au.status, relaunched, tt.Texts())
	}
	// Checking again does nothing once an update waits for the restart.
	a.checkAppUpdate(true)
	if f.checks != 4 {
		t.Fatalf("checked while ready: %d", f.checks)
	}
}

func TestAppUpdateDialog(t *testing.T) {
	a := newTestApp(t)
	a.au.version = "0.1.0"
	f := &fakeUpdater{release: newRelease(errors.New("signature mismatch"))}
	a.au.updater = f
	a.settings.AppUpdates.Beta = true
	tt := ui.NewTester(a.View, 1440, 900)

	// A background check offers the version in the dialog.
	a.checkAppUpdate(false)
	tt.Frame()
	if !f.beta || !a.au.dialog {
		t.Fatalf("beta %v, dialog %v", f.beta, a.au.dialog)
	}
	for _, s := range []string{"Dalibre 0.2.0 is available", "WHAT’S NEW", "Series shelves: books in a series stand together", "Faster import of large folders", "Skip this version", "Later"} {
		if !tt.HasText(s) {
			t.Fatalf("dialog misses %q: %q", s, tt.Texts())
		}
	}
	shot(t, tt, "app-update-dialog")

	// Not again within a day; skipped, never automatically.
	a.au.dialog = false
	a.checkAppUpdate(false)
	if f.checks != 1 {
		t.Fatalf("checked again within a day: %d", f.checks)
	}
	a.au.dialog = true
	tt.Frame()
	if err := tt.Click("Skip this version"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.au.dialog || a.settings.AppUpdates.Skipped != "0.2.0" {
		t.Fatalf("skip: dialog %v, skipped %q", a.au.dialog, a.settings.AppUpdates.Skipped)
	}
	a.settings.AppUpdates.Checked = time.Time{}
	a.checkAppUpdate(false)
	if a.au.dialog {
		t.Fatal("skipped version offered again")
	}

	// Installing fails: nothing changes, and it can be tried again.
	a.au.dialog = true
	a.installAppUpdate()
	tt.Frame()
	if a.au.status != auInstallFailed || !tt.HasText("signature mismatch") || !tt.HasText("Download from GitHub") {
		t.Fatalf("install failed: %v %q", a.au.status, tt.Texts())
	}

	// Manual turns the background checks off.
	a.settings.AppUpdates.Manual = true
	a.settings.AppUpdates.Checked = time.Time{}
	n := f.checks
	a.checkAppUpdate(false)
	if f.checks != n {
		t.Fatal("checked with automatic checks off")
	}
}

func TestReleaseNotes(t *testing.T) {
	got := releaseNotes("# 1.0\n\n- one\n* two `x`\n\n### Fixes\n+ three\n", 2)
	if len(got) != 2 || got[0] != "one" || got[1] != "two x" {
		t.Fatalf("%q", got)
	}
}
