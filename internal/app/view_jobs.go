package app

import (
	"slices"

	"github.com/egoist/mygo/ui"

	"dalibre/cmd/dalibre/locales"
	"dalibre/internal/app/icons"
	"dalibre/internal/app/theme"
	"dalibre/internal/app/widgets"
	"dalibre/internal/calibre"
)

// jobsButton opens the list of downloads, from the
// sidebar's footer.
func (a *App) jobsButton(c *ui.Context) {
	failed, downloading := 0, 0
	for _, d := range a.downloads {
		switch {
		case !d.finished:
			downloading++
		case d.err != nil && !isCanceled(d.err):
			failed++
		}
	}
	total := len(a.downloads)
	b := ui.ButtonBase(c).Padding(theme.Space6, theme.Space8).Gap(theme.Space10).Radius(theme.RadiusSM).AlignItems(ui.Center).Label(locales.JobsTitle())
	if b.Hovered() || a.jobsOpen {
		b.Background(pal().Hover)
	}
	if b.Clicked() {
		a.jobsOpen = !a.jobsOpen
	}
	b.Children(func() {
		switch {
		case downloading > 0:
			ui.Spinner(c).Size(16, 16)
		case failed > 0:
			ui.Icon(c, icons.Alert).FontSize(16).TextColor(pal().Danger.FG)
		default:
			ui.Icon(c, icons.CheckCircle).FontSize(16).TextColor(pal().Success.FG)
		}
		label := locales.JobsCount(total)
		switch {
		case downloading > 0:
			label = locales.JobsDownloadingOf(len(a.downloads)-downloading+1, len(a.downloads))
		case failed > 0:
			label = locales.JobsFailedCount(failed)
		}
		ui.Text(c, label).FontSize(12).FontWeight(500).Grow(1).SingleLine()
		ui.Icon(c, icons.ChevronRight).FontSize(12).TextColor(pal().IconMuted)
	})
	ui.PopoverBase(c, b, &a.jobsOpen, func(panel ui.Element) {
		panel.Width(380).Padding(theme.Space8).Gap(theme.Space4).Radius(theme.RadiusLG).Background(pal().SurfaceRaised).Border(1, pal().Border).
			Shadow(0, 2, 6, 0, ui.RGBA(26, 23, 18, 0.08)).Shadow(0, 24, 48, -8, ui.RGBA(26, 23, 18, 0.18)).AlignItems(ui.Stretch).Children(func() {
			ui.Row(c).Padding(theme.Space4, theme.Space4, theme.Space4, theme.Space8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, locales.JobsTitle()).FontSize(14).FontWeight(600).Grow(1)
				clear := ui.ButtonBase(c).Padding(theme.Space2, theme.Space6).Radius(theme.RadiusXS).Label(locales.JobsClearFinished())
				if clear.Hovered() {
					clear.Background(pal().Hover)
				}
				clear.OnClick(func() {
					a.downloads = slices.DeleteFunc(a.downloads, func(d *download) bool { return d.finished })
					if len(a.downloads) == 0 {
						a.jobsOpen = false
					}
				}).Children(func() {
					ui.Text(c, locales.JobsClearFinished()).FontSize(12).FontWeight(500).TextColor(pal().TextAccent)
				})
			})
			ui.Scroll(c).MaxHeight(420).Gap(theme.Space4).AlignItems(ui.Stretch).Children(func() {
				for i := len(a.downloads) - 1; i >= 0; i-- {
					a.downloadItem(c.Key(a.downloads[i]), a.downloads[i])
				}
			})
		})
	})
}

// downloadItem is the JobItem component for a book coming from a source.
func (a *App) downloadItem(c *ui.Context, d *download) {
	src := d.plugin.Manifest.ID
	ui.Row(c).Padding(theme.Space10, theme.Space12).Gap(theme.Space12).Radius(theme.RadiusMD).AlignItems(ui.Center).Children(func() {
		coverOf(c, &calibre.Book{Title: d.result.Title, Authors: d.result.Authors}, a.resultCover(src, d.result), 28, 40, true)
		ui.Column(c).Grow(1).MinWidth(0).Gap(theme.Space6).AlignItems(ui.Stretch).Children(func() {
			ui.Row(c).Gap(theme.Space8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, d.result.Title).FontSize(13).FontWeight(500).Grow(1).SingleLine()
				ui.Text(c, d.format).Font(theme.FontMono).FontSize(11).TextColor(pal().TextTertiary)
			})
			if !d.finished {
				ui.Progress(c, d.fraction()).Height(4)
			}
			ui.Row(c).Gap(theme.Space4).AlignItems(ui.Center).Children(func() {
				ic, col, msg := icons.Loader, pal().TextAccent, locales.JobsDownloadingProgress(d.progressText())
				switch {
				case d.finished && d.err == nil:
					ic, col, msg = icons.CheckCircle, pal().Success.FG, locales.JobsAddedToLibrary()
				case d.finished && isCanceled(d.err):
					ic, col, msg = icons.X, pal().IconMuted, locales.JobsCanceled()
				case d.finished:
					ic, col, msg = icons.Alert, pal().Danger.FG, pluginErrorText(a, d.result.Title, d.err)
				}
				ui.Icon(c, ic).FontSize(12).TextColor(col)
				t := ui.Text(c, msg).FontSize(12).TextColor(pal().TextSecondary).MaxLines(2)
				if d.finished && d.err != nil && !isCanceled(d.err) {
					t.TextColor(pal().Danger.FG)
				}
			})
		})
		switch {
		case !d.finished:
			if widgets.IconButton(c, icons.X, locales.CommonCancel(), false).Clicked() {
				d.cancel()
			}
		case d.err != nil:
			if widgets.IconButton(c, icons.Refresh, locales.CommonTryAgain(), false).Clicked() {
				a.downloads = slices.DeleteFunc(a.downloads, func(x *download) bool { return x == d })
				a.downloadBook(d.plugin, d.result, d.format)
			}
		case d.bookID != 0:
			if widgets.IconButton(c, icons.Library, locales.JobsShowInLibrary(), false).Clicked() {
				a.jobsOpen = false
				a.showInLibrary(d.bookID)
			}
		}
	})
}
