package uidevice

import (
	"testing"

	"github.com/worldiety/gift/geom"
)

func TestDisplayScale(t *testing.T) {
	cases := []struct {
		name    string
		px, mm  geom.Size
		scale   float32
		compact bool
	}{
		{"Pi 7 Zoll, erste Generation", geom.Sz(800, 480), geom.Sz(154, 86), 1, true},
		{"7 Zoll 1024x600 ohne Maße", geom.Sz(1024, 600), geom.Sz(0, 0), 1, true},
		{"7 Zoll 1024x600", geom.Sz(1024, 600), geom.Sz(154, 86), 1.25, true},
		{"Touch Display 2, 7 Zoll", geom.Sz(1280, 720), geom.Sz(155, 88), 1.5, true},
		{"Touch Display 2, 5 Zoll", geom.Sz(1280, 720), geom.Sz(111, 62), 1.5, true},
		{"Touch Display 2 hochkant gemeldet", geom.Sz(1280, 720), geom.Sz(88, 155), 1.5, true},
		{"10 Zoll 1280x800", geom.Sz(1280, 800), geom.Sz(217, 136), 1.15, false},
		{"Full-HD 15,6 Zoll", geom.Sz(1920, 1080), geom.Sz(344, 194), 1.05, false},
		{"Fernseher", geom.Sz(1920, 1080), geom.Sz(1600, 900), 1.5, false},
		{"Fenster am Schreibtisch", geom.Sz(1280, 720), geom.Sz(0, 0), 1, false},
	}

	for _, c := range cases {
		d := display{viewport: c.px, physical: c.mm}
		if got := d.scale(); got != c.scale || d.compact() != c.compact {
			t.Errorf("%s: scale %v compact %v, want %v %v", c.name, got, d.compact(), c.scale, c.compact)
		}
	}

	// Mit Dichte 2 zeichnet gift doppelt so groß; der Rest bleibt für u().
	d := display{viewport: geom.Sz(640, 360), physical: geom.Sz(111, 62), density: 2}
	if d.scale() != 0.75 || !d.compact() {
		t.Errorf("density 2: scale %v compact %v, want 0.75 true", d.scale(), d.compact())
	}

	if got := (display{viewport: geom.Sz(1280, 720), fixed: 2}).scale(); got != 2 {
		t.Errorf("fixed scale ignored: %v", got)
	}
}
