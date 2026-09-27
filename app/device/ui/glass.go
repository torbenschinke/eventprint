package uidevice

import (
	"github.com/worldiety/gift"
	"github.com/worldiety/gift/geom"
	"github.com/worldiety/gift/ui"
)

// Glas
//
// Die Flächen auf Home-Bildschirm und Druck-Studio sind Glas im Sinne von
// ui.Glass: Sie zeigen weichgezeichnet, was hinter ihnen liegt, mit einer
// hellen Kante oben links. Hinter ihnen liegt das Hintergrundbild, und das
// macht sie billig; siehe wallpaper.go.
//
// "Transparenz reduzieren" in den Einstellungen macht sie deckend, wie die
// Karten vor dem Glas. Das ist lesbarer bei grellem Licht und spart auf
// einem langsamen Pi die letzten Millisekunden.

// solid ist "Transparenz reduzieren", tint die Tönung von 0 (klar) bis 1
// (getönt); beides gesetzt von ApplyTheme.
var (
	solid bool
	tint  float32 = defaultTint
)

// defaultTint entspricht der Milch der Vorlage: 24 % Weiß im Hellen.
const defaultTint = 0.3

// tintOf liest die Tönung aus den Einstellungen.
func tintOf(percent int) float32 {
	if percent <= 0 {
		return defaultTint
	}

	return float32(min(percent, 100)) / 100
}

// paneTint ist die Milch des Glases, paneEdge seine helle Kante. Die
// Tönung reicht von fast klar bis fast deckend.
func paneTint() ui.Color {
	if darkPalette {
		return ui.RGBA(30, 30, 36, uint8(30+200*tint))
	}

	return ui.RGBA(255, 255, 255, uint8(15+215*tint))
}

func paneEdge() ui.Color {
	if darkPalette {
		return ui.RGBA(255, 255, 255, 40)
	}

	return ui.RGBA(255, 255, 255, 110)
}

// paneBackground ist Glas oder, bei reduzierter Transparenz, die Fläche.
func paneBackground(tint ui.Color) ui.Background {
	if solid {
		return ui.ColorSurface
	}

	// Flach wie iOS 26/27: Milch und Unschärfe, keine Lichtbrechung am
	// Rand. Die Kante sind zwei Haarlinien, die gift zeichnet: außen dunkel,
	// innen ein Glanz, der oben links hell ist und unten rechts kaum.
	return ui.Glass().Tint(tint).Blur(u(26)).Refraction(0).Highlight(0.9).Grain(0.03)
}

// paneShadow hebt eine Glasfläche leicht vom Hintergrund ab.
func paneShadow() ui.Shadow {
	return ui.Shadow{Blur: u(30), OffsetY: u(10), Color: ui.RGBA(0, 0, 0, 36)}
}

// glassCard ist eine Karte aus Glas; sie ersetzt card auf Glasbildschirmen.
func glassCard(children ...gift.View) ui.Stack {
	return glassPane(u(pick(28, 20)), paneTint(), children...).
		Padding(u(pick(22, 14))).
		Gap(u(pick(12, 8)))
}

// glassPane ist eine Glasfläche mit Radius und Tönung.
func glassPane(radius float32, tint ui.Color, children ...gift.View) ui.Stack {
	s := ui.VStack(children...).
		Background(paneBackground(tint)).
		CornerRadius(radius).
		Shadow(paneShadow())
	if solid {
		s = s.Border(ui.Border{Width: 1, Color: ui.ColorSeparator})
	}

	return s
}

// glassPill ist eine Kapsel aus Glas um einen Inhalt, etwa in der
// Statusleiste.
func glassPill(height float32, content ...gift.View) ui.Stack {
	s := ui.HStack(content...).
		Gap(u(7)).
		Align(geom.Center).
		PaddingInsets(geom.Insets{Left: u(14), Right: u(14)}).
		MinHeight(height).
		Background(paneBackground(paneTint())).
		CornerRadius(height / 2)
	if solid {
		s = s.Border(ui.Border{Width: 1, Color: ui.ColorSeparator})
	}

	return s
}

// caps ist die kleine Überschrift in Großbuchstaben über einer Glaskarte.
func caps(s string) ui.TextView {
	return ui.Text(s).FontSize(u(pick(12, 11))).Font(boldFont).Foreground(ui.ColorSecondaryLabel)
}

// edgeFade ist der Scroll-Rand von iPadOS 26: Unter einer schwebenden
// Kopfzeile verschwindet der Inhalt nicht an einer harten Kante, sondern
// wird weich ausgeblendet. Die Kopfzeile selbst (solid hoch) bleibt fast
// deckend, darunter läuft die Deckung über tail auf null aus.
//
// gift kennt keine Verlaufsfüllung; ein Dutzend gestaffelter Streifen sieht
// auf einem ruhigen Hintergrund genauso aus und kostet einen Zeichenaufruf.
func edgeFade(solid, tail float32, c ui.Color) gift.View {
	const steps = 24
	bands := make([]gift.View, 0, steps+1)
	bands = append(bands, ui.Box().Frame(geom.Unbounded(), solid*0.8).Background(c))
	rest := solid*0.2 + tail
	for i := range steps {
		t := (float32(i) + 0.5) / steps
		bands = append(bands, ui.Box().Frame(geom.Unbounded(), rest/steps).Background(ui.Fade(c, 1-t*t*(3-2*t))))
	}

	return ui.VStack(bands...)
}
