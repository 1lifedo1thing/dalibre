// Command dalibre is a desktop e-book manager that reads and writes
// calibre libraries.
package main

import (
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"dalibre/internal/app"
	"dalibre/internal/app/theme"
	"dalibre/internal/settings"
)

func main() {
	if err := theme.RegisterFonts(); err != nil {
		log.Fatal(err)
	}
	a := app.New(settings.DefaultPath())
	mygo.App.WhenReady(func() {
		win := mygo.NewWindow(mygo.WindowOptions{
			Title:           "Dalibre",
			Width:           1440,
			Height:          900,
			MinWidth:        960,
			MinHeight:       600,
			StateKey:        "main",
			TitleBarStyle:   mygo.TitleBarHiddenInset,
			TitleBarHeight:  48,
			BackgroundColor: "light-dark(#f9f9f8, #111110)",
			Content:         ui.View(a.View),
		})
		a.Attach(win)
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
