package app

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"dalibre/internal/app/icons"
	"dalibre/internal/app/theme"
	"dalibre/internal/app/widgets"
)

// dropOverlay marks the window as a drop target while files are dragged
// over it. The dragged paths aren't known until the drop, so it can't
// count them yet.
func (a *App) dropOverlay(c *ui.Context) {
	ui.Box(c).Absolute().Top(theme.Space12).Left(theme.Space12).Right(theme.Space12).Bottom(theme.Space12).PassThrough().
		Radius(theme.RadiusXL).Border(2, pal().Accent).BorderStyle(ui.BorderDashed).Background(pal().Accent.Alpha(0.06)).Center().Children(func() {
		ui.Column(c).Width(400).Center().Gap(theme.Space12).Padding(theme.Space24, theme.Space32).Radius(theme.RadiusLG).
			Background(pal().SurfaceRaised).Border(1, pal().BorderSubtle).Shadow(0, 2, 6, 0, ui.RGBA(26, 23, 18, 0.08)).Shadow(0, 24, 48, -8, ui.RGBA(26, 23, 18, 0.18)).Children(func() {
			ui.Box(c).Size(48, 48).Radius(theme.RadiusFull).Background(pal().AccentSubtle).Center().Children(func() {
				ui.Icon(c, icons.Upload).FontSize(22).TextColor(pal().TextAccent)
			})
			ui.Text(c, "Drop to add books").FontSize(18).FontWeight(600)
			hint := "Folders are searched for e-books. Books you already have are checked before they’re added."
			if a.busy != "" {
				hint = "They’ll be added after the books being added now."
			}
			ui.Text(c, hint).FontSize(12).TextColor(pal().TextTertiary).TextAlign(ui.Center)
		})
	})
}

// importReviewSheet asks what to do with dropped books the library has
// already, once the rest are added, and lists the files that failed.
func (a *App) importReviewSheet(c *ui.Context) {
	im := &a.imp
	if !im.review {
		return
	}
	n := len(im.held)
	widgets.DialogFrame(c, &im.review, 600, func() {
		ui.Column(c).Grow(1).Gap(theme.Space4).Children(func() {
			title := fmt.Sprintf("%d books are already in your library", n)
			if n == 1 {
				title = "This book is already in your library"
			}
			ui.Text(c, title).FontSize(16).FontWeight(600)
			sub := "Choose what to do with " + plural(n, "it.", "these.")
			if added := len(im.added); added > 0 {
				sub = fmt.Sprintf("%d of the %d books were added. ", added, im.files) + sub
			}
			ui.Text(c, sub).FontSize(13).TextColor(pal().TextSecondary)
		})
	}, func() {
		ui.Column(c).Padding(theme.Space4, theme.Space24, theme.Space16, theme.Space24).AlignItems(ui.Stretch).Children(func() {
			for _, h := range im.held {
				a.heldRow(c.Key(h), h)
			}
			if len(im.failed) > 0 || len(im.skipped) > 0 {
				ui.Column(c).Padding(theme.Space16, 0, 0, 0).Gap(theme.Space8).AlignItems(ui.Stretch).Children(func() {
					widgets.Overline(c, "COULDN’T ADD")
					for _, f := range im.failed {
						name, why, _ := strings.Cut(f, ": ")
						problemRow(c.Key("f:"+f), icons.Alert, pal().Danger.FG, name, why)
					}
					for _, s := range im.skipped {
						problemRow(c.Key("s:"+s), icons.File, pal().IconMuted, s, "not an e-book")
					}
				})
			}
		})
	}, func() {
		ui.Spacer(c)
		if widgets.Button(c, widgets.Secondary, nil, "Skip all").Disabled(im.applying).Clicked() {
			a.skipReview()
		}
		if widgets.Button(c, widgets.Primary, nil, "Apply").Disabled(im.applying).Clicked() {
			a.applyReview()
		}
	})
	if !im.review && !im.applying { // closed with ✕ or Esc
		a.skipReview()
	}
}

// heldRow is the ImportDuplicateRow component.
func (a *App) heldRow(c *ui.Context, h *heldBook) {
	b := h.dup.Book
	ui.Row(c).Padding(theme.Space10, 0).Gap(theme.Space12).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(pal().BorderSubtle).Children(func() {
		a.coverView(c, b, 28, 40, true)
		ui.Column(c).Grow(1).MinWidth(0).Gap(theme.Space2).Children(func() {
			ui.Text(c, b.Title).FontSize(13).FontWeight(500).SingleLine()
			ui.Text(c, h.detail()).FontSize(12).TextColor(pal().TextSecondary).SingleLine()
		})
		sel := ui.ButtonBase(c).Width(200).Height(32).Padding(0, theme.Space8, 0, theme.Space10).Gap(theme.Space6).AlignItems(ui.Center).
			Radius(theme.RadiusSM).Background(pal().Surface).Border(1, pal().Border).Label("What to do with " + b.Title)
		if sel.Hovered() {
			sel.Border(1, pal().BorderStrong)
		}
		sel.PopoverMenuButton(func(m *ui.Menu) {
			for _, act := range h.actions() {
				if m.Item(h.label(act)).ID(fmt.Sprint("a:", act)).Radio(act == h.action).Chosen() {
					h.action = act
				}
			}
		})
		sel.Children(func() {
			ui.Text(c, h.label(h.action)).FontSize(13).Grow(1).SingleLine()
			ui.Icon(c, icons.ChevronDown).FontSize(12).TextColor(pal().IconMuted)
		})
	})
}

func problemRow(c *ui.Context, ic *ui.SVG, col ui.Color, name, why string) {
	ui.Row(c).Gap(theme.Space8).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, ic).FontSize(14).TextColor(col)
		ui.Text(c, name+" — "+why).FontSize(13).TextColor(pal().TextSecondary).Grow(1).MinWidth(0).SingleLine()
	})
}
