package diskfree_test

import (
	"testing"

	"github.com/torbenschinke/eventprint/pkg/diskfree"
)

// TestGiBMatchesWhatAFileManagerShows: Zweierpotenzen, nicht Zehnerpotenzen.
// Eine Zahl, die von der im Dateimanager abweicht, stiftet nur Zweifel.
func TestGiBMatchesWhatAFileManagerShows(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{bytes: 0, want: "0.00 GiB"},
		{bytes: 1024 * 1024 * 1024, want: "1.00 GiB"},
		{bytes: 1536 * 1024 * 1024, want: "1.50 GiB"},

		// 10^9 sind knapp 0,93 GiB. Wer hier 1,00 anzeigte, rechnete in
		// Gigabyte und wiche vom Betriebssystem ab.
		{bytes: 1_000_000_000, want: "0.93 GiB"},
	}

	for _, tt := range tests {
		if got := diskfree.GiB(tt.bytes); got != tt.want {
			t.Errorf("GiB(%d) = %q, erwartet %q", tt.bytes, got, tt.want)
		}
	}
}

// TestUsedIsTotalMinusFree haelt die einzige Rechnung fest.
func TestUsedIsTotalMinusFree(t *testing.T) {
	u := diskfree.Usage{TotalBytes: 1000, FreeBytes: 300}

	if got := u.UsedBytes(); got != 700 {
		t.Fatalf("UsedBytes = %d, erwartet 700", got)
	}
}

// TestOfMeasuresARealPath ist die Gegenprobe gegen das Betriebssystem: Ohne
// sie waere nicht belegt, dass die Naht ueberhaupt etwas liefert.
func TestOfMeasuresARealPath(t *testing.T) {
	u, err := diskfree.Of(t.TempDir())
	if err != nil {
		t.Fatalf("Of: %v", err)
	}

	if u.TotalBytes <= 0 {
		t.Fatalf("TotalBytes = %d, erwartet mehr als 0", u.TotalBytes)
	}

	if u.FreeBytes < 0 || u.FreeBytes > u.TotalBytes {
		t.Fatalf("FreeBytes = %d passt nicht zu TotalBytes = %d", u.FreeBytes, u.TotalBytes)
	}
}

// TestOfFailsLoudlyOnAMissingPath: Ein stiller Nullwert saehe aus wie eine
// leere Platte.
func TestOfFailsLoudlyOnAMissingPath(t *testing.T) {
	if _, err := diskfree.Of("/gibt/es/nicht/hoffentlich"); err == nil {
		t.Fatal("ein nicht vorhandener Pfad wurde ohne Fehler gemessen")
	}
}
