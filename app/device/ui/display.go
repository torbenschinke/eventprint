package uidevice

import (
	"fmt"
	"math"

	"github.com/worldiety/gift/geom"
)

// Die Box läuft an sehr verschiedenen Bildschirmen: am alten 7-Zoll-Panel mit
// 800 x 480, an 7-Zoll-Panels mit 1024 x 600, am Raspberry Pi Touch Display 2
// mit 1280 x 720 auf 5 oder 7 Zoll, an 10-Zoll-Panels und an Full-HD. Eine
// feste Vergrößerung passt zu keinem davon; die Oberfläche wird deshalb aus
// Auflösung und Pixeldichte des tatsächlichen Bildschirms bemessen.
//
// Gerechnet wird in Entwurfspunkten, die u() in Bildschirmpunkte umrechnet.
// Ein Entwurfspunkt soll ungefähr so groß sein wie ein Punkt auf einem iPad
// (132 ppi): Dann ist ein Knopf von 44 Punkten gut 8 mm hoch und mit dem
// Finger sicher zu treffen, gleich auf welchem Panel.

// designPPI ist die Dichte, bei der ein Entwurfspunkt ein Pixel ist.
const designPPI = 132

// Der kleinste Entwurf, für den die Bildschirme gebaut sind. Kleiner wird nie
// gerechnet, auch wenn das Panel dafür eigentlich zu dicht ist: Dann werden
// die Knöpfe eben kleiner als 8 mm, aber nichts fällt aus dem Bild.
const (
	minDesignWidth  = 760
	minDesignHeight = 480
)

// Unterhalb dieser Maße gilt die kompakte Größenklasse: Seitenleisten werden
// zu Leisten über dem Inhalt, Listen und Details zu zwei Bildschirmen.
const (
	compactWidth  = 1100
	compactHeight = 640
)

// display ist die Bemessung der Oberfläche.
type display struct {
	// viewport ist die Fläche in Bildschirmpunkten.
	viewport geom.Size

	// physical ist die Größe des Panels in Millimetern, sofern bekannt.
	physical geom.Size

	// fixed ist eine von außen vorgegebene Vergrößerung; 0 heißt
	// automatisch.
	fixed float32

	// density ist die ganzzahlige Dichte, mit der gift zeichnet: Pixel je
	// Bildschirmpunkt. Die Anzeigesitzung setzt sie auf dichten Panels auf
	// 2, damit auch die Bausteine von gift mit festen Maßen – Tastatur,
	// Listenzeilen, Schalter – mitwachsen. scale ist dann nur noch der Rest.
	density float32
}

// dens ist die Dichte, mindestens 1.
func (d display) dens() float64 {
	if d.density < 1 {
		return 1
	}

	return float64(d.density)
}

// scale liefert den Faktor von Entwurfs- zu Bildschirmpunkten.
//
// Gerechnet wird in Bildschirmpunkten, also nach der Dichte, die gift ohnehin
// anwendet; nur die Pixeldichte des Panels zählt echte Pixel.
func (d display) scale() float32 {
	vw, vh := float64(d.viewport.W), float64(d.viewport.H)
	if vw <= 0 || vh <= 0 {
		return 1
	}

	if d.fixed > 0 {
		return d.fixed
	}

	// Ohne verlässliche Größe – billige Panels melden oft keine oder die
	// eines Fernsehers – nach der Höhe: bis 720 Zeilen 1, darüber mehr.
	s := math.Max(1, vh/720)

	if ppi, ok := d.ppi(); ok {
		s = ppi / designPPI / d.dens()
	}

	// Mindestens der kleinste Entwurf muss auf das Panel passen; lieber
	// kleinere Knöpfe als abgeschnittene. Größer als nötig wird nur, wer
	// dicht genug ist, und nie mehr als dreifach.
	hi := math.Min(vw/minDesignWidth, vh/minDesignHeight)
	lo := math.Min(1, hi)
	s = math.Min(math.Max(s, lo), math.Max(lo, hi))
	s = math.Min(s, 3)

	// Auf Zwanzigstel runden: Zwei Panels, die sich um ein Pixel
	// unterscheiden, sollen nicht verschieden aussehen.
	return float32(math.Round(s*20) / 20)
}

// ppi ist die Pixeldichte, wenn die gemeldete Größe glaubwürdig ist.
func (d display) ppi() (float64, bool) {
	if d.physical.W <= 0 || d.physical.H <= 0 {
		return 0, false
	}

	w, h := float64(d.physical.W), float64(d.physical.H)

	// Ein gedrehtes Panel meldet seine Maße oft ungedreht.
	if (d.viewport.W > d.viewport.H) != (w > h) {
		w, h = h, w
	}

	ppi := float64(d.viewport.W) * d.dens() / (w / 25.4)

	// Fernseher melden Zentimeter statt Millimeter, manche Panels 1 x 1 oder
	// 160 x 90: Was keine Bildschirmdichte sein kann, wird nicht geglaubt.
	return ppi, ppi >= 80 && ppi <= 420
}

// compact meldet die kompakte Größenklasse.
func (d display) compact() bool {
	if d.viewport.W <= 0 || d.viewport.H <= 0 {
		return false
	}

	s := d.scale()
	return float32(d.viewport.W)/s < compactWidth || float32(d.viewport.H)/s < compactHeight
}

// String beschreibt die Bemessung für das Protokoll.
func (d display) String() string {
	return fmt.Sprintf("viewport=%vx%v density=%v mm=%vx%v scale=%v compact=%v", d.viewport.W, d.viewport.H, d.dens(), d.physical.W, d.physical.H, d.scale(), d.compact())
}
