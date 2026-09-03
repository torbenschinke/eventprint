package cfgphotobox

import (
	"testing"

	"github.com/torbenschinke/eventprint/app/printing"
)

// Diese Entscheidungen liefen bisher nur im Browsertest mit, obwohl an ihnen
// nichts vom Browser abhängt: Es sind Regeln darüber, wann eine Vorbelegung
// aus der Umgebung eine bestehende Einstellung überschreiben darf – und wann
// eben nicht.

func TestPrinterDefaultNeverOverwritesAChoice(t *testing.T) {
	tests := []struct {
		name    string
		opts    Options
		current printing.Settings
		want    string
		changed bool
	}{
		{
			name:    "leere Einstellung übernimmt die Umgebung",
			opts:    Options{PrinterQueue: "CZ01"},
			want:    "CZ01",
			changed: true,
		},
		{
			name:    "Leerraum wird abgeschnitten",
			opts:    Options{PrinterQueue: "  CZ01\n"},
			want:    "CZ01",
			changed: true,
		},
		{
			name:    "ohne Umgebung bleibt alles, wie es ist",
			opts:    Options{},
			current: printing.Settings{Queue: "CZ01"},
			want:    "CZ01",
		},
		{
			name:    "eine gesetzte Warteschlange wird nicht überschrieben",
			opts:    Options{PrinterQueue: "AUS-DER-UMGEBUNG"},
			current: printing.Settings{Queue: "VON-HAND"},
			want:    "VON-HAND",
		},
		{
			name: "ohne beides bleibt es beim Testbetrieb",
			opts: Options{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, changed := withPrinterDefault(tt.opts, tt.current)

			if got.Queue != tt.want {
				t.Errorf("Queue = %q, erwartet %q", got.Queue, tt.want)
			}

			if changed != tt.changed {
				t.Errorf("changed = %v, erwartet %v", changed, tt.changed)
			}
		})
	}
}

func TestBoothDefaultNeverOverwritesATitle(t *testing.T) {
	got, changed := withBoothDefault(Options{EventTitle: " Hochzeit "}, Settings{})
	if !changed || got.EventTitle != "Hochzeit" {
		t.Fatalf("EventTitle = %q, changed = %v", got.EventTitle, changed)
	}

	got, changed = withBoothDefault(Options{EventTitle: "Aus der Umgebung"}, Settings{EventTitle: "Von Hand"})
	if changed || got.EventTitle != "Von Hand" {
		t.Fatalf("ein gesetzter Titel wurde überschrieben: %q", got.EventTitle)
	}
}

// TestCameraDefaultsApplyOnlyOnce hält den Merker fest.
//
// Ohne ihn wäre eine bewusst abgeschaltete Automatik nicht von "noch nie
// eingestellt" zu unterscheiden, und sie spränge beim nächsten Start wieder
// an – auf einer Feier ein Drucker, der von selbst zu arbeiten beginnt.
func TestCameraDefaultsApplyOnlyOnce(t *testing.T) {
	first, changed := withCameraDefaults(Settings{})
	if !changed {
		t.Fatal("die Erstbelegung wurde nicht gesetzt")
	}

	if !first.CameraAutoPrint || first.CameraTemplate != printing.TemplatePolaroid {
		t.Fatalf("Erstbelegung = %+v", first)
	}

	// Die Betreuung schaltet die Automatik ab.
	off := first
	off.CameraAutoPrint = false

	again, changed := withCameraDefaults(off)
	if changed || again.CameraAutoPrint {
		t.Fatal("die abgeschaltete Automatik wurde wieder eingeschaltet")
	}
}

func TestAutoCropDefaultsApplyOnlyOnce(t *testing.T) {
	first, changed := withAutoCropDefaults(Settings{})
	if !changed || !first.AutoCrop {
		t.Fatalf("Erstbelegung = %+v, changed = %v", first, changed)
	}

	off := first
	off.AutoCrop = false

	again, changed := withAutoCropDefaults(off)
	if changed || again.AutoCrop {
		t.Fatal("der abgeschaltete Bildausschnitt wurde wieder eingeschaltet")
	}
}
