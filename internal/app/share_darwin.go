//go:build darwin

package app

import (
	"github.com/ebitengine/purego/objc"

	"github.com/egoist/mygo"
)

// shareSupported reports whether shareFiles shows the system's share sheet.
const shareSupported = true

type nsPoint struct{ X, Y float64 }
type nsSize struct{ Width, Height float64 }
type nsRect struct {
	Origin nsPoint
	Size   nsSize
}

const nsMinYEdge = 1

// sharePicker is the picker on screen; AppKit doesn't keep it alive while it
// shows, so it is held here until the next one replaces it.
var sharePicker objc.ID

func sel(s string) objc.SEL { return objc.RegisterName(s) }

// shareFiles shows the system's share sheet for the files at paths; tests
// replace it.
var shareFiles = nativeShare

// nativeShare shows macOS's share picker (NSSharingServicePicker) for the
// files at paths, under the pointer in win.
func nativeShare(win *mygo.Window, paths []string) {
	if win == nil || len(paths) == 0 {
		return
	}
	mygo.RunOnMain(func() {
		nswin := objc.ID(win.NativeHandle())
		if nswin == 0 {
			return
		}
		view := nswin.Send(sel("contentView"))
		items := objc.ID(objc.GetClass("NSMutableArray")).Send(sel("array"))
		for _, p := range paths {
			s := objc.ID(objc.GetClass("NSString")).Send(sel("stringWithUTF8String:"), p+"\x00")
			items.Send(sel("addObject:"), objc.ID(objc.GetClass("NSURL")).Send(sel("fileURLWithPath:"), s))
		}
		if sharePicker != 0 {
			sharePicker.Send(sel("release"))
		}
		sharePicker = objc.ID(objc.GetClass("NSSharingServicePicker")).Send(sel("alloc")).Send(sel("initWithItems:"), items)
		at := objc.Send[nsPoint](nswin, sel("mouseLocationOutsideOfEventStream"))
		at = objc.Send[nsPoint](view, sel("convertPoint:fromView:"), at, objc.ID(0))
		sharePicker.Send(sel("showRelativeToRect:ofView:preferredEdge:"), nsRect{at, nsSize{1, 1}}, view, uint(nsMinYEdge))
	})
}
