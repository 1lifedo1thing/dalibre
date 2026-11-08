package app

import (
	"fmt"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"dalibre/internal/app/icons"
	"dalibre/internal/app/theme"
	"dalibre/internal/app/widgets"
	"dalibre/internal/calibre"
	"dalibre/internal/convert"
)

type convertState struct {
	ids     []int64
	opts    convert.Options
	profile string // the profile's title, as the select shows it
	engine  convert.Engine
	found   bool
}

func (a *App) openConvert(ids []int64) {
	if len(ids) == 0 {
		return
	}
	engine, found := convert.FindEngine()
	opts := a.convert.opts
	if opts.Output == "" {
		opts = convert.DefaultOptions()
	}
	a.convert = convertState{ids: ids, opts: opts, profile: profileTitle(opts.Profile), engine: engine, found: found}
	a.convertOpen = true
}

func profileTitle(id string) string {
	for _, p := range convert.Profiles {
		if p.ID == id {
			return p.Title
		}
	}
	return convert.Profiles[0].Title
}

func profileID(title string) string {
	for _, p := range convert.Profiles {
		if p.Title == title {
			return p.ID
		}
	}
	return convert.Profiles[0].ID
}

func (a *App) convertDialog(c *ui.Context) {
	if !a.convertOpen {
		return
	}
	s := &a.convert
	var titles, sources []string
	for _, id := range s.ids {
		if b := a.byID[id]; b != nil {
			titles = append(titles, b.Title)
			if src := convert.SourceFormat(b.Formats, s.opts.Output); src != "" && !contains(sources, src) {
				sources = append(sources, src)
			}
		}
	}
	n := len(s.ids)
	widgets.DialogFrame(c, &a.convertOpen, 600, func() {
		ui.Box(c).Size(32+float32(min(n, 3)-1)*14, 40).Children(func() {
			for i, id := range s.ids[:min(n, 3)] {
				if b := a.byID[id]; b != nil {
					ui.Box(c.Key(id)).Absolute().Left(float32(i)*14).Top(0).Border(2, pal().SurfaceRaised).Radius(4).Children(func() {
						a.coverView(c, b, 28, 40, true)
					})
				}
			}
		})
		ui.Column(c).Grow(1).MinWidth(0).Gap(theme.Space2).Children(func() {
			ui.Textf(c, "Convert %d %s", n, plural(n, "book", "books")).FontWeight(600).FontSize(18)
			ui.Text(c, joinTitles(titles)).FontSize(13).TextColor(pal().TextSecondary).SingleLine()
		})
	}, func() {
		ui.Column(c).Padding(theme.Space24).Gap(theme.Space24).AlignItems(ui.Stretch).Children(func() {
			ui.Column(c).Gap(theme.Space12).AlignItems(ui.Stretch).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Gap(theme.Space6).Children(func() {
					ui.Text(c, "Output format").FontSize(14).FontWeight(600).Grow(1)
					if len(sources) > 0 {
						ui.Text(c, "From").FontSize(12).TextColor(pal().TextTertiary)
						for _, src := range sources {
							widgets.FormatBadge(c.Key(src), src, false)
						}
					}
				})
				ui.Row(c).Gap(theme.Space12).AlignItems(ui.Stretch).Children(func() {
					for _, f := range convert.Formats {
						if widgets.FormatOption(c.Key(f.Name), f.Name, f.Title, f.Description, s.opts.Output == f.Name).Clicked() {
							s.opts.Output = f.Name
						}
					}
				})
			})
			ui.Column(c).Gap(theme.Space16).AlignItems(ui.Stretch).Children(func() {
				ui.Row(c).Gap(theme.Space16).Children(func() {
					widgets.Field(c, "Device profile", "", func() {
						titles := make([]string, len(convert.Profiles))
						for i, p := range convert.Profiles {
							titles[i] = p.Title
						}
						if ui.Select(c, &s.profile, titles).Label("Device profile").Changed() {
							s.opts.Profile = profileID(s.profile)
						}
					})
					if s.opts.Output == "PDF" {
						widgets.Field(c, "Paper size", "", func() {
							ui.Select(c, &s.opts.PaperSize, []string{"a4", "a5", "a6", "letter"}).Label("Paper size")
						})
					}
				})
				ui.Row(c).Gap(theme.Space24).Wrap().Children(func() {
					ui.Checkbox(c, &s.opts.EmbedFonts, "Embed fonts").Width(250)
					ui.Checkbox(c, &s.opts.KeepTOC, "Keep table of contents").Width(250)
					ui.Checkbox(c, &s.opts.KeepOriginal, "Keep original files").Width(250)
				})
			})
			// The engine: what runs the conversion, and whether it's there.
			ui.Row(c).Padding(theme.Space10, theme.Space12).Gap(theme.Space10).Radius(theme.RadiusMD).Background(pal().Subtle).AlignItems(ui.Center).Children(func() {
				if s.found {
					ui.Icon(c, icons.CheckCircle).FontSize(16).TextColor(pal().Success.FG)
				} else {
					ui.Icon(c, icons.Alert).FontSize(16).TextColor(pal().Warning.FG)
				}
				ui.Column(c).Grow(1).MinWidth(0).Gap(theme.Space2).Children(func() {
					if s.found {
						ui.Text(c, "Engine: "+s.engine.Name).FontSize(13).FontWeight(500)
						ui.Text(c, s.engine.Path).Font(theme.FontMono).FontSize(11).TextColor(pal().TextTertiary).SingleLine()
					} else {
						ui.Text(c, "calibre’s ebook-convert wasn’t found").FontSize(13).FontWeight(500)
						ui.Text(c, "Install calibre to convert books. Conversion arrives in the next version.").FontSize(12).TextColor(pal().TextTertiary)
					}
				})
			})
		})
	}, func() {
		ui.Text(c, "Runs in the background").FontSize(12).TextColor(pal().TextTertiary).Grow(1)
		if widgets.Button(c, widgets.Secondary, nil, "Cancel").Clicked() {
			a.convertOpen = false
		}
		if widgets.Button(c, widgets.Primary, icons.Convert, fmt.Sprintf("Convert %d %s", n, plural(n, "book", "books"))).Clicked() {
			a.startConversions(s.ids, s.opts)
			a.convertOpen = false
		}
	})
}

func joinTitles(ts []string) string {
	switch len(ts) {
	case 0:
		return ""
	case 1:
		return ts[0]
	case 2:
		return ts[0] + " and " + ts[1]
	}
	return strings.Join(ts[:2], ", ") + fmt.Sprintf(" and %d more", len(ts)-2)
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// startConversions queues a job per book and runs them in the background.
func (a *App) startConversions(ids []int64, opts convert.Options) {
	var jobs []*convert.Job
	for _, id := range ids {
		b := a.byID[id]
		if b == nil {
			continue
		}
		j := &convert.Job{BookID: id, Title: b.Title, From: convert.SourceFormat(b.Formats, opts.Output), To: opts.Output}
		if j.From == "" {
			j.Status, j.Message = convert.Failed, "No file to convert from"
		}
		jobs = append(jobs, j)
	}
	a.jobs = append(a.jobs, jobs...)
	a.jobsOpen = true
	a.run(func() {
		for _, j := range jobs {
			if j.Status == convert.Failed {
				continue
			}
			a.post(func() { j.Status = convert.Running })
			err := convert.Run(j, opts)
			a.post(func() {
				if err != nil {
					j.Status, j.Message = convert.Failed, err.Error()
				} else {
					j.Status, j.Progress = convert.Done, 1
				}
			})
		}
	})
}

// jobsButton opens the list of downloads and conversions, from the
// sidebar's footer.
func (a *App) jobsButton(c *ui.Context) {
	running, failed, downloading := 0, 0, 0
	for _, j := range a.jobs {
		switch j.Status {
		case convert.Queued, convert.Running:
			running++
		case convert.Failed:
			failed++
		}
	}
	for _, d := range a.downloads {
		switch {
		case !d.finished:
			downloading++
		case d.err != nil && !isCanceled(d.err):
			failed++
		}
	}
	total := len(a.jobs) + len(a.downloads)
	b := ui.ButtonBase(c).Padding(theme.Space6, theme.Space8).Gap(theme.Space10).Radius(theme.RadiusSM).AlignItems(ui.Center).Label("Jobs")
	if b.Hovered() || a.jobsOpen {
		b.Background(pal().Hover)
	}
	if b.Clicked() {
		a.jobsOpen = !a.jobsOpen
	}
	b.Children(func() {
		switch {
		case running+downloading > 0:
			ui.Spinner(c).Size(16, 16)
		case failed > 0:
			ui.Icon(c, icons.Alert).FontSize(16).TextColor(pal().Danger.FG)
		default:
			ui.Icon(c, icons.CheckCircle).FontSize(16).TextColor(pal().Success.FG)
		}
		label := fmt.Sprintf("%d %s", total, plural(total, "job", "jobs"))
		switch {
		case downloading > 0:
			label = fmt.Sprintf("Downloading %d of %d", len(a.downloads)-downloading+1, len(a.downloads))
		case running > 0:
			label = fmt.Sprintf("Converting %d of %d", len(a.jobs)-running+1, len(a.jobs))
		case failed > 0:
			label = fmt.Sprintf("%d failed", failed)
		}
		ui.Text(c, label).FontSize(12).FontWeight(500).Grow(1).SingleLine()
		ui.Icon(c, icons.ChevronRight).FontSize(12).TextColor(pal().IconMuted)
	})
	ui.PopoverBase(c, b, &a.jobsOpen, func(panel ui.Element) {
		panel.Width(380).Padding(theme.Space8).Gap(theme.Space4).Radius(theme.RadiusLG).Background(pal().SurfaceRaised).Border(1, pal().Border).
			Shadow(0, 2, 6, 0, ui.RGBA(26, 23, 18, 0.08)).Shadow(0, 24, 48, -8, ui.RGBA(26, 23, 18, 0.18)).AlignItems(ui.Stretch).Children(func() {
			ui.Row(c).Padding(theme.Space4, theme.Space4, theme.Space4, theme.Space8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "Jobs").FontSize(14).FontWeight(600).Grow(1)
				clear := ui.ButtonBase(c).Padding(theme.Space2, theme.Space6).Radius(theme.RadiusXS).Label("Clear finished")
				if clear.Hovered() {
					clear.Background(pal().Hover)
				}
				clear.OnClick(func() {
					var keep []*convert.Job
					for _, j := range a.jobs {
						if j.Status == convert.Queued || j.Status == convert.Running {
							keep = append(keep, j)
						}
					}
					a.jobs = keep
					a.downloads = slices.DeleteFunc(a.downloads, func(d *download) bool { return d.finished })
					if len(a.jobs)+len(a.downloads) == 0 {
						a.jobsOpen = false
					}
				}).Children(func() { ui.Text(c, "Clear finished").FontSize(12).FontWeight(500).TextColor(pal().TextAccent) })
			})
			ui.Scroll(c).MaxHeight(420).Gap(theme.Space4).AlignItems(ui.Stretch).Children(func() {
				for i := len(a.downloads) - 1; i >= 0; i-- {
					a.downloadItem(c.Key(a.downloads[i]), a.downloads[i])
				}
				for i, j := range a.jobs {
					a.jobItem(c.Key(i), j)
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
				ic, col, msg := icons.Loader, pal().TextAccent, "Downloading · "+d.progressText()
				switch {
				case d.finished && d.err == nil:
					ic, col, msg = icons.CheckCircle, pal().Success.FG, "Added to library"
				case d.finished && isCanceled(d.err):
					ic, col, msg = icons.X, pal().IconMuted, "Canceled"
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
			if widgets.IconButton(c, icons.X, "Cancel", false).Clicked() {
				d.cancel()
			}
		case d.err != nil:
			if widgets.IconButton(c, icons.Refresh, "Try again", false).Clicked() {
				a.downloads = slices.DeleteFunc(a.downloads, func(x *download) bool { return x == d })
				a.downloadBook(d.plugin, d.result, d.format)
			}
		case d.bookID != 0:
			if widgets.IconButton(c, icons.Library, "Show in library", false).Clicked() {
				a.jobsOpen = false
				a.showInLibrary(d.bookID)
			}
		}
	})
}

// jobItem is the JobItem component.
func (a *App) jobItem(c *ui.Context, j *convert.Job) {
	ui.Row(c).Padding(theme.Space10, theme.Space12).Gap(theme.Space12).Radius(theme.RadiusMD).AlignItems(ui.Center).Children(func() {
		if b := a.byID[j.BookID]; b != nil {
			a.coverView(c, b, 28, 40, true)
		}
		ui.Column(c).Grow(1).MinWidth(0).Gap(theme.Space6).AlignItems(ui.Stretch).Children(func() {
			ui.Row(c).Gap(theme.Space8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, j.Title).FontSize(13).FontWeight(500).Grow(1).SingleLine()
				ui.Text(c, j.Label()).Font(theme.FontMono).FontSize(11).TextColor(pal().TextTertiary)
			})
			if j.Status == convert.Running {
				ui.Progress(c, -1).Height(4)
			}
			ui.Row(c).Gap(theme.Space4).AlignItems(ui.Center).Children(func() {
				ic, col, msg := icons.Clock, pal().IconMuted, "Queued"
				switch j.Status {
				case convert.Running:
					ic, col, msg = icons.Loader, pal().TextAccent, "Converting…"
				case convert.Done:
					ic, col, msg = icons.CheckCircle, pal().Success.FG, "Done"
				case convert.Failed:
					ic, col, msg = icons.Alert, pal().Danger.FG, "Failed · "+j.Message
				}
				ui.Icon(c, ic).FontSize(12).TextColor(col)
				t := ui.Text(c, msg).FontSize(12).TextColor(pal().TextSecondary).MaxLines(2)
				if j.Status == convert.Failed {
					t.TextColor(pal().Danger.FG)
				}
			})
		})
	})
}
