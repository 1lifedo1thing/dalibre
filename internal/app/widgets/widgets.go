// Package widgets holds the components of the Figma library, built on the
// toolkit's bases so they keep focus, keyboard activation and
// accessibility.
package widgets

import (
	"github.com/egoist/mygo/ui"

	"dalibre/internal/app/icons"
	"dalibre/internal/app/theme"
)

type Variant int

const (
	Secondary Variant = iota
	Primary
	Ghost
	Danger
)

// Button is the Button component: a label with an optional leading icon.
func Button(c *ui.Context, v Variant, icon *ui.SVG, label string) ui.Element {
	return ButtonSized(c, v, icon, label, theme.ControlM)
}

func ButtonSized(c *ui.Context, v Variant, icon *ui.SVG, label string, height float32) ui.Element {
	b := ui.ButtonBase(c).Height(height).Radius(theme.RadiusSM).Gap(theme.Space6).AlignItems(ui.Center).Justify(ui.Center).Label(label)
	pad := float32(theme.Space12)
	if height < theme.ControlM {
		pad = theme.Space10
	}
	b.Padding(0, pad)
	if icon != nil {
		b.PaddingStart(pad - 2)
	}
	var fg, bg ui.Color
	switch v {
	case Primary:
		fg, bg = pal().TextOnAccent, pal().Accent
		if b.Hovered() {
			bg = pal().AccentHover
		}
		b.Shadow(0, 1, 2, 0, ui.RGBA(26, 23, 18, 0.08))
	case Danger:
		fg, bg = pal().TextOnAccent, pal().DangerSolid
		if b.Hovered() {
			bg = pal().DangerSolid.Mix(ui.RGB(0, 0, 0), 0.1)
		}
	case Ghost:
		fg = pal().Text
		if b.Hovered() {
			bg = pal().Hover
		}
	default:
		fg, bg = pal().Text, pal().Surface
		border := pal().Border
		if b.Hovered() {
			bg, border = pal().Hover, pal().BorderStrong
		}
		b.Border(1, border).Shadow(0, 1, 2, 0, ui.RGBA(26, 23, 18, 0.05))
	}
	if b.Pressed() {
		b.Opacity(0.88)
	}
	b.Background(bg).TextColor(fg).Children(func() {
		if icon != nil {
			ui.Icon(c, icon).FontSize(16)
		}
		if label != "" {
			fs := float32(13)
			if height < theme.ControlM {
				fs = 12
			}
			ui.Text(c, label).FontSize(fs).FontWeight(500).SingleLine()
		}
	})
	return b
}

// IconButton is the IconButton component, with a tooltip as its name.
func IconButton(c *ui.Context, icon *ui.SVG, label string, active bool) ui.Element {
	b := ui.ButtonBase(c).Size(theme.ControlM, theme.ControlM).Radius(theme.RadiusSM).Center().Label(label).Tooltip(label)
	fg := pal().Icon
	switch {
	case active:
		b.Background(pal().Selected)
		fg = pal().TextAccent
	case b.Hovered():
		b.Background(pal().Hover)
		fg = pal().Text
	}
	b.TextColor(fg).Children(func() { ui.Icon(c, icon).FontSize(16) })
	return b
}

// FormatBadge is the FormatBadge component.
func FormatBadge(c *ui.Context, format string, small bool) ui.Element {
	col := pal().Format(format)
	e := ui.Row(c).Radius(theme.RadiusXS).Background(col.BG)
	fs := float32(10)
	if small {
		e.Padding(theme.Space2, theme.Space4)
		fs = 9
	} else {
		e.Padding(theme.Space2+1, theme.Space6)
	}
	return e.Children(func() {
		ui.Text(c, format).Font(theme.FontMono).FontWeight(500).FontSize(fs).LetterSpacing(0.4).TextColor(col.FG)
	})
}

// Tag is the Tag component. It reports a click on the chip, and onRemove
// adds a remove Button.
func Tag(c *ui.Context, label string, selected bool, onRemove func()) ui.Element {
	e := ui.ButtonBase(c).Height(24).Radius(theme.RadiusFull).AlignItems(ui.Center).Gap(theme.Space4).Label(label)
	e.Padding(0, theme.Space8)
	if onRemove != nil {
		e.PaddingEnd(theme.Space4)
	}
	bg, fg := pal().Subtle, pal().TextSecondary
	switch {
	case selected:
		bg, fg = pal().AccentSubtle, pal().TextAccent
	case e.Hovered():
		bg = pal().Hover
	}
	return e.Background(bg).TextColor(fg).Children(func() {
		ui.Text(c, label).FontSize(12).FontWeight(500).SingleLine()
		if onRemove != nil {
			x := ui.ButtonBase(c).Size(16, 16).Radius(theme.RadiusFull).Center().Label("Remove " + label)
			if x.Hovered() {
				x.Background(fg.Alpha(0.15))
			}
			x.OnClick(onRemove).Children(func() { ui.Icon(c, icons.X).FontSize(11) })
		}
	})
}

// Stars shows a calibre rating (0–10) as five Stars.
func Stars(c *ui.Context, rating int, size float32) ui.Element {
	return ui.Row(c).Gap(theme.Space2).Label(ratingLabel(rating)).Children(func() {
		for i := 1; i <= 5; i++ {
			if rating >= i*2 {
				ui.Icon(c, icons.Star).FontSize(size).TextColor(pal().WarningSolid)
			} else if rating == i*2-1 {
				ui.Icon(c, icons.Star).FontSize(size).TextColor(pal().WarningSolid.Alpha(0.5))
			} else {
				ui.Icon(c, icons.StarOutline).FontSize(size).TextColor(pal().BorderStrong)
			}
		}
	})
}

func ratingLabel(r int) string {
	if r == 0 {
		return "Not rated"
	}
	return string(rune('0'+r/2)) + " Stars"
}

// Overline is a small uppercase section heading.
func Overline(c *ui.Context, s string) ui.Element {
	return ui.Text(c, s).FontSize(11).FontWeight(600).LetterSpacing(0.6).TextColor(pal().TextTertiary)
}

// Hairline is a divider across its column.
func Hairline(c *ui.Context) ui.Element {
	return ui.Box(c).Height(1).Background(pal().BorderSubtle)
}

// Card is a raised surface, as the Menu and the dialogs.
func Card(c *ui.Context) ui.Element {
	return ui.Column(c).Background(pal().SurfaceRaised).Border(1, pal().BorderSubtle).Radius(theme.RadiusLG).
		Shadow(0, 2, 6, 0, ui.RGBA(26, 23, 18, 0.08)).Shadow(0, 24, 48, -8, ui.RGBA(26, 23, 18, 0.18))
}

func pal() *theme.Palette { return theme.P() }

// DialogFrame is a dialog in the design's look: a dimmed window, a raised
// panel of the given width, a header with a close button, and a footer.
func DialogFrame(c *ui.Context, open *bool, width float32, header func(), body func(), footer func()) {
	ui.DialogBase(c, open, func(backdrop, panel ui.Element) {
		backdrop.Background(pal().Scrim)
		panel.Width(width).MaxHeight(ui.Auto).Background(pal().SurfaceRaised).Border(1, pal().BorderSubtle).Radius(theme.RadiusXL).Clip().
			Shadow(0, 2, 6, 0, ui.RGBA(26, 23, 18, 0.08)).Shadow(0, 24, 48, -8, ui.RGBA(26, 23, 18, 0.24)).AlignItems(ui.Stretch)
		panel.Children(func() {
			ui.Row(c).Padding(theme.Space16, theme.Space16, theme.Space16, theme.Space24).Gap(theme.Space12).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(pal().BorderSubtle).Children(func() {
				header()
				if IconButton(c, icons.X, "Close", false).Clicked() {
					*open = false
				}
			})
			ui.Scroll(c).Grow(1).MinHeight(0).Children(body)
			ui.Row(c).Padding(theme.Space16, theme.Space24).Gap(theme.Space8).AlignItems(ui.Center).Background(pal().BgApp).BorderWidth(1, 0, 0, 0).BorderColor(pal().BorderSubtle).Children(footer)
		})
	})
}

// Field is a labeled control, with a hint at the label's end.
func Field(c *ui.Context, label, hint string, control func()) ui.Element {
	return ui.Column(c.Key(label)).Grow(1).Basis(0).MinWidth(0).Gap(theme.Space6).AlignItems(ui.Stretch).Children(func() {
		ui.Row(c).Children(func() {
			ui.Text(c, label).FontSize(12).FontWeight(500).TextColor(pal().TextSecondary).Grow(1)
			if hint != "" {
				ui.Text(c, hint).FontSize(12).TextColor(pal().TextTertiary)
			}
		})
		control()
	})
}

// FormatOption is the FormatOption component: a radio card choosing an
// output format.
func FormatOption(c *ui.Context, name, title, description string, selected bool) ui.Element {
	e := ui.ButtonBase(c).Grow(1).Basis(0).Padding(theme.Space16).Radius(theme.RadiusMD).AlignItems(ui.Stretch).
		Role(ui.RoleRadio).Label(name + ", " + title)
	switch {
	case selected:
		e.Background(pal().Selected).Border(1.5, pal().BorderSelected)
	case e.Hovered():
		e.Background(pal().Hover).Border(1, pal().BorderStrong)
	default:
		e.Background(pal().Surface).Border(1, pal().Border)
	}
	return e.Children(func() {
		ui.Column(c).Grow(1).Gap(theme.Space12).AlignItems(ui.Stretch).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				FormatBadge(c, name, false)
				ui.Spacer(c)
				r := ui.Box(c).Size(16, 16).Radius(theme.RadiusFull).Center()
				if selected {
					r.Background(pal().Accent).Children(func() {
						ui.Box(c).Size(6, 6).Radius(theme.RadiusFull).Background(pal().TextOnAccent)
					})
				} else {
					r.Background(pal().Surface).Border(1, pal().BorderStrong)
				}
			})
			ui.Column(c).Gap(theme.Space4).Children(func() {
				ui.Text(c, title).FontSize(13).FontWeight(500)
				ui.Text(c, description).FontSize(12).TextColor(pal().TextSecondary)
			})
		})
	})
}
