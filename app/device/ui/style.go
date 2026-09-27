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
	"log/slog"
	"strings"

	"github.com/torbenschinke/eventprint/pkg/xgift"
)

// scale vergrößert alle Maße, compact wählt die kompakten Anordnungen. Beides
// folgt aus dem Bildschirm, siehe display.go und [App.Fit], und ändert sich
// nur zwischen zwei Aufbauten der Oberfläche.
var (
	scale   float32 = 1
	compact bool

	// design ist die Fläche in Entwurfspunkten; leer heißt 1280 x 720, der
	// Entwurf, mit dem die Tests laufen.
	design geom.Size
)

// vw und vh sind Breite und Höhe der Fläche in Entwurfspunkten. Bildschirme,
// die ihre Anordnung nach dem Platz richten, rechnen damit.
func vw() float32 {
	if design.W <= 0 {
		return 1280
	}

	return design.W
}

func vh() float32 {
	if design.H <= 0 {
		return 720
	}

	return design.H
}

// u rechnet ein Entwurfsmaß in logische Punkte um.
func u(v float32) float32 { return v * scale }

var (
	blue        = ui.RGB(0x0A, 0x66, 0xD9)
	green       = ui.RGB(0x1E, 0x8E, 0x3E)
	pink        = ui.RGB(0xC2, 0x18, 0x5B)
	purple      = ui.RGB(0x6E, 0x4B, 0xD8)
	grey        = ui.RGB(0x63, 0x63, 0x66)
	navy        = ui.RGB(0x24, 0x32, 0x4F)
	amber       = ui.RGB(0xE9, 0xB9, 0x49)
	ink         = ui.RGB(0x11, 0x11, 0x14)
	kioskBg     = ui.RGB(0x10, 0x10, 0x13)
	kioskCard   = ui.RGB(0x1C, 0x1C, 0x21)
	kioskRaised = ui.RGB(0x26, 0x26, 0x2C)
	kioskMuted  = ui.RGB(0xB4, 0xB4, 0xBB)
	white       = ui.OpaqueWhite
)

// Die Farben, die das Thema von gift nicht kennt. Sie hängen am
// Erscheinungsbild und werden mit ihm gesetzt, siehe [setPalette]; ein Wechsel
// baut die Oberfläche danach vollständig neu auf.
var (
	blueText   ui.Color // Text in Akzentblau auf Flächen
	blueWash   ui.Color // gewählte Karte
	greenText  ui.Color
	orange     ui.Color // Warnung als Text und Symbol
	red        ui.Color // Fehler, Löschen
	wallpaper  ui.Color // Hintergrund des Home-Bildschirms
	canvas     ui.Color // Tisch unter der Druckvorschau
	notice     ui.Color // Hinweisfläche, warm
	dangerWash ui.Color // Fläche hinter einer zerstörenden Aktion
	tileA      ui.Color // Platzhalter der Kacheln
	tileB      ui.Color
	tileError  ui.Color
	kioskPanel ui.Color // die Kiosk-Karte auf dem Home-Bildschirm
)

func init() { setPalette(false) }

// setPalette setzt die Sonderfarben für hell oder dunkel.
// darkPalette ist das Erscheinungsbild, für das setPalette zuletzt gesetzt
// hat.
var darkPalette bool

func setPalette(dark bool) {
	darkPalette = dark
	if dark {
		blueText = ui.RGB(0x4C, 0x9D, 0xFF)
		blueWash = ui.RGB(0x14, 0x2A, 0x48)
		greenText = ui.RGB(0x4C, 0xD9, 0x64)
		orange = ui.RGB(0xFF, 0x9F, 0x2E)
		red = ui.RGB(0xFF, 0x5A, 0x52)
		wallpaper = ui.RGB(0x16, 0x15, 0x14)
		canvas = ui.RGB(0x10, 0x10, 0x12)
		notice = ui.RGB(0x3A, 0x2E, 0x14)
		dangerWash = ui.RGB(0x42, 0x1C, 0x1C)
		tileA, tileB = ui.RGB(0x2C, 0x2C, 0x30), ui.RGB(0x34, 0x34, 0x39)
		tileError = ui.RGB(0x5A, 0x24, 0x24)
		kioskPanel = ui.RGB(0x2A, 0x2A, 0x30)

		return
	}

	blueText = ui.RGB(0x0A, 0x5B, 0xC4)
	blueWash = ui.RGB(0xEE, 0xF4, 0xFD)
	greenText = ui.RGB(0x1E, 0x7A, 0x36)
	orange = ui.RGB(0xB8, 0x53, 0x00)
	red = ui.RGB(0xC4, 0x16, 0x1C)
	wallpaper = ui.RGB(0xE7, 0xE4, 0xDF)
	canvas = ui.RGB(0xE9, 0xE9, 0xEE)
	notice = ui.RGB(0xFF, 0xF6, 0xE0)
	dangerWash = ui.RGB(0xFD, 0xEC, 0xEC)
	tileA, tileB = ui.RGB(0xE3, 0xE3, 0xE8), ui.RGB(0xDA, 0xDA, 0xE0)
	tileError = ui.RGB(0xF3, 0xC6, 0xC6)
	kioskPanel = ui.RGB(0x16, 0x16, 0x1A)
}

// homeTheme ist das Erscheinungsbild des Heimbetriebs, hell oder dunkel wie
// die Einstellungen am iPad.
func homeTheme(dark bool) ui.Theme {
	if dark {
		return ui.DarkTheme().
			With(ui.ColorAccent, ui.RGB(0x0A, 0x84, 0xFF)).
			With(ui.ColorOnAccent, white).
			With(ui.ColorBackground, ui.RGB(0x0B, 0x0B, 0x0D)).
			With(ui.ColorSurface, ui.RGB(0x1C, 0x1C, 0x1E))
	}

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

// pick wählt ein Entwurfsmaß nach der Größenklasse.
func pick(regular, small float32) float32 {
	if compact {
		return small
	}

	return regular
}

// pick2 wählt einen Text nach der Größenklasse: Auf kleinen Panels reicht
// oft das erste Wort.
func pick2(regular, small string) string {
	if compact {
		return small
	}

	return regular
}

// clamp begrenzt v auf [lo, hi].
func clamp(v, lo, hi float32) float32 { return max(lo, min(v, hi)) }

// gutter ist der Abstand der Bildschirminhalte vom Rand.
func gutter() float32 { return pick(40, 16) }

// spacing ist der Abstand zwischen Karten.
func spacing() float32 { return pick(20, 12) }

// card ist eine abgerundete Fläche auf dem Hintergrund.
func card(children ...gift.View) ui.Stack {
	return ui.VStack(children...).
		Background(ui.ColorSurface).
		CornerRadius(u(pick(22, 16))).
		Padding(u(pick(22, 14))).
		Gap(u(pick(14, 10)))
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
// capsule ist der Radius einer Kapsel der Höhe h: Knöpfe sind seit iOS 26
// vollständig abgerundet.
func capsule(h float32) float32 { return u(h / 2) }

func filled(label string, face, fg ui.Color, action func()) ui.ButtonView {
	r := capsule(pick(52, 44))
	return ui.Button(ui.Text(label).FontSize(u(pick(17, 16))).Font(boldFont).Foreground(fg), action).
		Style(ui.ButtonStyle{Background: face, Border: noBorder, CornerRadius: r}).
		HoverStyle(ui.ButtonStyle{Background: face, Border: noBorder, CornerRadius: r}).
		PressedStyle(ui.ButtonStyle{Background: ui.Fade(face, 0.75), CornerRadius: r}).
		DisabledStyle(ui.ButtonStyle{Background: ui.Fade(face, 0.35), CornerRadius: r}).
		MinHeight(u(pick(52, 44))).
		PaddingInsets(geom.Insets{Left: u(pick(22, 16)), Right: u(pick(22, 16))})
}

// secondary ist eine Nebenaktion auf heller Fläche.
func secondary(label string, action func()) ui.ButtonView {
	face := ui.Fade(ui.ColorAccent, 0.12)
	r := capsule(pick(48, 44))
	return ui.Button(ui.Text(label).FontSize(u(16)).Font(boldFont).Foreground(ui.ColorAccent), action).
		Style(ui.ButtonStyle{Background: face, Border: noBorder, CornerRadius: r}).
		HoverStyle(ui.ButtonStyle{Background: face, Border: noBorder, CornerRadius: r}).
		PressedStyle(ui.ButtonStyle{Background: ui.Fade(ui.ColorAccent, 0.25), CornerRadius: r}).
		MinHeight(u(pick(48, 44))).
		PaddingInsets(geom.Insets{Left: u(pick(18, 14)), Right: u(pick(18, 14))})
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
	face := ui.ButtonStyle{Background: ui.Fade(ui.ColorLabel, 0.05), Border: noBorder, CornerRadius: u(24)}
	return ui.Button(ui.Icon(sym).Size(u(22)).Foreground(fg), action).
		Style(face).HoverStyle(face).
		PressedStyle(ui.ButtonStyle{Background: ui.Fade(ui.ColorLabel, 0.15), CornerRadius: u(24)}).
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
	return ui.VStack(v).PaddingInsets(geom.Insets{Bottom: u(pick(24, 12)), Left: u(12), Right: u(12)})
}

// humane macht aus einem Fehler eine Zeile für Menschen. Fehler von
// Programmen wie nmcli oder lsblk tragen Befehlszeilen und Ausgaben, die auf
// dem Bildschirm niemandem helfen und über den Rand laufen; sie landen im
// Protokoll.
func humane(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "executable file not found"):
		slog.Warn("tool missing", "err", err)
		return "Auf diesem Gerät nicht verfügbar."
	case strings.Contains(msg, "exit status") || strings.Contains(msg, "exec:"):
		slog.Warn("tool failed", "err", err)
		return "Das hat nicht geklappt. Einzelheiten stehen im Protokoll."
	default:
		return msg
	}
}
