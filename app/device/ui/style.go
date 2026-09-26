// Package uidevice ist die Oberfläche des Geräts auf dem Touchscreen.
//
// Sie hat zwei Gesichter. Im Heimbetrieb ist sie hell und fühlt sich an wie
// ein Tablet: Home-Bildschirm, Fotos, Druck-Studio, Einstellungen. Im Kiosk
// ist sie dunkel und kennt nur die Feier. Der Wechsel der Farbe ist Absicht:
// Wer vor der Box steht, sieht auf einen Blick, in welcher Betriebsart sie ist.
package uidevice

import (
	"github.com/worldiety/gift"
	"github.com/worldiety/gift/font/inter"
	"github.com/worldiety/gift/geom"
	"github.com/worldiety/gift/ui"

	"github.com/torbenschinke/eventprint/pkg/xgift"
)

// scale vergrößert alle Maße. Der Entwurf rechnet mit 1280 x 720 logischen
// Punkten; ein 1080p-Bildschirm mit Skalierungsfaktor 1 braucht 1,5.
var scale float32 = 1

// SetScale legt den Vergrößerungsfaktor fest. Aufzurufen vor dem ersten Bild.
func SetScale(s float32) {
	if s > 0 {
		scale = s
	}
}

// u rechnet ein Entwurfsmaß in logische Punkte um.
func u(v float32) float32 { return v * scale }

var (
	blue        = ui.RGB(0x0A, 0x66, 0xD9)
	blueText    = ui.RGB(0x0A, 0x5B, 0xC4)
	blueWash    = ui.RGB(0xE8, 0xF0, 0xFC)
	green       = ui.RGB(0x1E, 0x8E, 0x3E)
	greenText   = ui.RGB(0x1E, 0x7A, 0x36)
	orange      = ui.RGB(0xB8, 0x53, 0x00)
	red         = ui.RGB(0xC4, 0x16, 0x1C)
	pink        = ui.RGB(0xC2, 0x18, 0x5B)
	purple      = ui.RGB(0x6E, 0x4B, 0xD8)
	grey        = ui.RGB(0x63, 0x63, 0x66)
	navy        = ui.RGB(0x24, 0x32, 0x4F)
	amber       = ui.RGB(0xE9, 0xB9, 0x49)
	ink         = ui.RGB(0x11, 0x11, 0x14)
	wallpaper   = ui.RGB(0xE7, 0xE4, 0xDF)
	kioskBg     = ui.RGB(0x10, 0x10, 0x13)
	kioskCard   = ui.RGB(0x1C, 0x1C, 0x21)
	kioskRaised = ui.RGB(0x26, 0x26, 0x2C)
	kioskMuted  = ui.RGB(0xB4, 0xB4, 0xBB)
	white       = ui.OpaqueWhite
)

// homeTheme ist das helle Erscheinungsbild des Heimbetriebs.
func homeTheme() ui.Theme {
	return ui.LightTheme().
		With(ui.ColorAccent, blue).
		With(ui.ColorOnAccent, white).
		With(ui.ColorBackground, ui.RGB(0xF2, 0xF2, 0xF7)).
		With(ui.ColorSurface, white)
}

// kioskTheme ist das dunkle Erscheinungsbild der Feier in ihrer Akzentfarbe.
func kioskTheme(accent ui.Color) ui.Theme {
	return ui.DarkTheme().
		With(ui.ColorAccent, accent).
		With(ui.ColorOnAccent, ink).
		With(ui.ColorBackground, kioskBg).
		With(ui.ColorSurface, kioskCard)
}

var boldFont = ui.MustFont(ui.FontQuery{Family: inter.Family, Weight: ui.WeightBold})

// title ist fetter Text in Entwurfsgröße.
func title(s string, size float32) ui.TextView {
	return ui.Text(s).FontSize(u(size)).Font(boldFont)
}

// body ist normaler Text in Entwurfsgröße.
func body(s string, size float32) ui.TextView {
	return ui.Text(s).FontSize(u(size))
}

// muted ist leiser Begleittext.
func muted(s string, size float32) ui.TextView {
	return ui.Text(s).FontSize(u(size)).Foreground(ui.ColorSecondaryLabel)
}

// card ist eine weiße, abgerundete Fläche auf dem Hintergrund.
func card(children ...gift.View) ui.Stack {
	return ui.VStack(children...).
		Background(ui.ColorSurface).
		CornerRadius(u(22)).
		Padding(u(22)).
		Gap(u(14))
}

// section ist eine gruppierte Liste mit Überschrift, wie in den Einstellungen.
func section(header string, rows ...gift.View) gift.View {
	items := []gift.View{}
	if header != "" {
		items = append(items, muted(header, 13).PaddingInsets(geom.Insets{Left: u(16), Top: u(8)}))
	}

	items = append(items, ui.VStack(ui.List(rows...).SeparatorInsets(u(16), 0)).
		Background(ui.ColorSurface).CornerRadius(u(14)).Clip(true))

	return ui.VStack(items...).Gap(u(6))
}

// primary ist die Hauptaktion eines Bildschirms.
func primary(label string, action func()) ui.ButtonView {
	return filled(label, ui.ColorAccent, ui.ColorOnAccent, action)
}

// filled ist ein flächiger Knopf in beliebiger Farbe.
func filled(label string, face, fg ui.Color, action func()) ui.ButtonView {
	return ui.Button(ui.Text(label).FontSize(u(17)).Font(boldFont).Foreground(fg), action).
		Style(ui.ButtonStyle{Background: face, Border: noBorder, CornerRadius: u(14)}).
		HoverStyle(ui.ButtonStyle{Background: face, Border: noBorder, CornerRadius: u(14)}).
		PressedStyle(ui.ButtonStyle{Background: ui.Fade(face, 0.75), CornerRadius: u(14)}).
		DisabledStyle(ui.ButtonStyle{Background: ui.Fade(face, 0.35), CornerRadius: u(14)}).
		MinHeight(u(52)).
		PaddingInsets(geom.Insets{Left: u(22), Right: u(22)})
}

// secondary ist eine Nebenaktion auf heller Fläche.
func secondary(label string, action func()) ui.ButtonView {
	face := ui.Fade(ui.ColorAccent, 0.12)
	return ui.Button(ui.Text(label).FontSize(u(16)).Font(boldFont).Foreground(ui.ColorAccent), action).
		Style(ui.ButtonStyle{Background: face, Border: noBorder, CornerRadius: u(14)}).
		HoverStyle(ui.ButtonStyle{Background: face, Border: noBorder, CornerRadius: u(14)}).
		PressedStyle(ui.ButtonStyle{Background: ui.Fade(ui.ColorAccent, 0.25), CornerRadius: u(14)}).
		MinHeight(u(48)).
		PaddingInsets(geom.Insets{Left: u(18), Right: u(18)})
}

// link ist ein Knopf, der nur aus Text besteht.
func link(label string, action func()) ui.ButtonView {
	clear := ui.ButtonStyle{Background: ui.ColorClear, Border: noBorder}
	return ui.Button(ui.Text(label).FontSize(u(16)).Foreground(ui.ColorAccent), action).
		Style(clear).HoverStyle(clear).
		PressedStyle(ui.ButtonStyle{Background: ui.Fade(ui.ColorAccent, 0.1), CornerRadius: u(10)}).
		MinHeight(ui.ControlHitTarget)
}

// iconButton ist ein quadratischer Knopf mit Symbol.
func iconButton(sym ui.Symbol, label string, fg ui.Color, action func()) ui.ButtonView {
	face := ui.ButtonStyle{Background: ui.Fade(ui.ColorLabel, 0.05), Border: noBorder, CornerRadius: u(12)}
	return ui.Button(ui.Icon(sym).Size(u(22)).Foreground(fg), action).
		Style(face).HoverStyle(face).
		PressedStyle(ui.ButtonStyle{Background: ui.Fade(ui.ColorLabel, 0.15), CornerRadius: u(12)}).
		Frame(u(48), u(48)).
		Label(label)
}

// noBorder schaltet die Haarlinie ab, die das Thema um Knöpfe zeichnet. Eine
// Linie ohne Breite gälte als "nicht gesetzt"; eine durchsichtige ist
// ausdrücklich keine.
var noBorder = ui.Border{Width: 1, Color: ui.ColorClear}

// fill ist ein dehnbarer Zwischenraum.
func fill() gift.View { return ui.Spacer() }

// grow lässt einen Inhalt den verfügbaren Platz füllen.
//
// Ein gift.Component reicht die Dehnbarkeit seines Inhalts nicht an den
// umgebenden Stack weiter; ohne diese Hülle bekäme ein Bildschirm nur seine
// Mindesthöhe.
func grow(v gift.View) xgift.FillView { return xgift.Fill(v).Flex(1) }

// floating hebt eine schwebende Leiste vom unteren Rand ab.
func floating(v gift.View) gift.View {
	return ui.VStack(v).PaddingInsets(geom.Insets{Bottom: u(24)})
}
