package printing_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/torbenschinke/eventprint/app/printing"
)

// TestSettingsPrinterFollowsSettings ist der Kern der Druckerauswahl: Eine
// Änderung in den Einstellungen muss ohne Neustart wirken, weil die Fotobox
// mitten auf einer Veranstaltung umkonfiguriert wird.
func TestSettingsPrinterFollowsSettings(t *testing.T) {
	current := printing.Settings{}

	p := printing.NewSettingsPrinter(func() printing.Settings { return current })

	if got := p.Name(); got != printing.TestModeName {
		t.Errorf("ohne Warteschlange = %q, erwartet %q", got, printing.TestModeName)
	}

	if !printing.IsTestMode(p) {
		t.Error("ohne Warteschlange muss der Testmodus aktiv sein")
	}

	current.Queue = "CZ01"

	if got := p.Name(); got != "CZ01" {
		t.Errorf("nach der Änderung = %q, erwartet %q", got, "CZ01")
	}

	if printing.IsTestMode(p) {
		t.Error("mit Warteschlange darf der Testmodus nicht mehr aktiv sein")
	}
}

// TestSettingsPrinterDiscardsInTestMode stellt sicher, dass im Testbetrieb
// wirklich kein Druckauftrag entsteht.
func TestSettingsPrinterDiscardsInTestMode(t *testing.T) {
	p := printing.NewSettingsPrinter(func() printing.Settings { return printing.Settings{} })

	res, err := p.Print(context.Background(), []byte("egal"), "test.jpg")
	if err != nil {
		t.Fatalf("Testmodus darf nicht fehlschlagen: %v", err)
	}

	if res.Message == "" {
		t.Error("erwartete eine Rückmeldung für die Oberfläche")
	}

	// Im Testbetrieb entsteht kein Auftrag, der sich nachverfolgen ließe.
	if outcome := printing.NewSettingsPrinter(func() printing.Settings { return printing.Settings{} }).(printing.Tracker).
		Await(context.Background(), res.JobID); !outcome.Success {
		t.Error("der Testbetrieb muss als erfolgreich gelten")
	}
}

// TestSettingsPrinterWithoutLoader schützt vor einer unvollständigen
// Verdrahtung: Ohne Einstellungsquelle darf nicht versehentlich gedruckt
// werden.
func TestSettingsPrinterWithoutLoader(t *testing.T) {
	p := printing.NewSettingsPrinter(nil)

	if !printing.IsTestMode(p) {
		t.Error("ohne Einstellungsquelle muss der Testmodus aktiv sein")
	}
}

// TestSettingsPrinterUsesUnknownQueue dokumentiert das Verhalten bei einem
// Tippfehler: Der Fehler von lp landet am Auftrag, statt still verschluckt zu
// werden.
func TestSettingsPrinterUsesUnknownQueue(t *testing.T) {
	p := printing.NewSettingsPrinter(func() printing.Settings {
		return printing.Settings{Queue: "gibt-es-nicht-4711"}
	})

	if _, err := p.Print(context.Background(), []byte{0xFF, 0xD8}, "test.jpg"); err == nil {
		t.Skip("lp meldet auf diesem System keinen Fehler für unbekannte Warteschlangen")
	} else if errors.Is(err, context.Canceled) {
		t.Fatalf("unerwarteter Abbruch: %v", err)
	}
}

// TestParseQueues prüft die Auswertung von "lpstat -a". Nur das erste Feld
// jeder Zeile ist stabil, der Rest ist lokalisierter Fließtext.
func TestParseQueues(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{
			name: "deutsch",
			in:   "CZ01 akzeptiert Anfragen seit Di 11 Aug 2026 19:09:42 CEST\n",
			want: []string{"CZ01"},
		},
		{
			name: "englisch",
			in:   "CZ01 accepting requests since Tue 11 Aug 2026 07:09:42 PM CEST\n",
			want: []string{"CZ01"},
		},
		{
			name: "mehrere, sortiert und ohne Dubletten",
			in:   "Officejet accepting requests\nCZ01 accepting requests\nCZ01 accepting requests\n",
			want: []string{"CZ01", "Officejet"},
		},
		{
			name: "leere Ausgabe",
			in:   "\n\n",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := printing.ParseQueues(tt.in)

			if len(got) != len(tt.want) {
				t.Fatalf("ParseQueues = %v, erwartet %v", got, tt.want)
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ParseQueues[%d] = %q, erwartet %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func hasOption(args []string, option string) bool { return slices.Contains(args, option) }

// TestDriverOptionsDefaultsToLowSpeed hält die Vorgabe fest.
//
// Bei normaler Geschwindigkeit wirken die Farben auf diesem Gerät sichtbar
// verwaschen: Der Thermokopf verweilt kürzer auf jeder Zeile und überträgt
// entsprechend weniger Farbe. Für eine Fotobox zählt das Ergebnis mehr als
// der Durchsatz, deshalb wird ohne ausdrückliche Wahl langsam gedruckt.
func TestDriverOptionsDefaultsToLowSpeed(t *testing.T) {
	args := printing.CUPSPrinter{Queue: "CZ01"}.DriverOptionsForTest()

	if !hasOption(args, "StpPrintSpeed=LowSpeed") {
		t.Errorf("StpPrintSpeed=LowSpeed fehlt in %v", args)
	}
}

// TestDriverOptionsHonourExplicitSpeed stellt sicher, dass die Einstellung auch
// wirklich durchschlägt – wer Durchsatz braucht, muss ihn bekommen.
func TestDriverOptionsHonourExplicitSpeed(t *testing.T) {
	args := printing.CUPSPrinter{Queue: "CZ01", PrintSpeed: printing.SpeedNormal}.DriverOptionsForTest()

	if !hasOption(args, "StpPrintSpeed=Normal") {
		t.Errorf("StpPrintSpeed=Normal fehlt in %v", args)
	}

	if hasOption(args, "StpPrintSpeed=LowSpeed") {
		t.Errorf("die Vorgabe überschreibt die ausdrückliche Wahl: %v", args)
	}
}

// TestDriverOptionsQualityDefaults sichert die übrigen Treiberoptionen ab, die für
// die Bildqualität entscheidend sind.
func TestDriverOptionsQualityDefaults(t *testing.T) {
	args := printing.CUPSPrinter{Queue: "CZ01"}.DriverOptionsForTest()

	// Ohne Photo verwendet der Treiber die Vorgabe TextGraphics, die für
	// Fotos die falsche Farbaufbereitung wählt.
	if !hasOption(args, "StpImageType=Photo") {
		t.Errorf("StpImageType=Photo fehlt in %v", args)
	}

	if !hasOption(args, "PageSize=w288h432") {
		t.Errorf("PageSize fehlt oder ist falsch in %v", args)
	}

}

// TestDriverOptionsOmitEmptyValues prüft, dass unbelegte Einstellungen dem
// Treiber überlassen bleiben statt ihn mit leeren Werten zu füttern.
func TestDriverOptionsOmitEmptyValues(t *testing.T) {
	args := printing.CUPSPrinter{Queue: "CZ01"}.DriverOptionsForTest()

	for _, value := range args {
		if len(value) > 0 && value[len(value)-1] == '=' {
			t.Errorf("leere Option %q in %v", value, args)
		}
	}

	if hasOption(args, "StpLaminate=") {
		t.Errorf("leeres Finish wird übergeben: %v", args)
	}
}

// TestDriverOptionsPassLaminate deckt das Oberflächenfinish ab.
func TestDriverOptionsPassLaminate(t *testing.T) {
	args := printing.CUPSPrinter{Queue: "CZ01", Laminate: "Matte"}.DriverOptionsForTest()

	if !hasOption(args, "StpLaminate=Matte") {
		t.Errorf("StpLaminate=Matte fehlt in %v", args)
	}
}

// TestSettingsSpeedDefault prüft die Einstellungsebene getrennt von der
// Argumentliste.
func TestSettingsSpeedDefault(t *testing.T) {
	if got := (printing.Settings{}).Speed(); got != printing.SpeedLow {
		t.Errorf("Speed() = %q, erwartet %q", got, printing.SpeedLow)
	}

	if got := (printing.Settings{PrintSpeed: printing.SpeedNormal}).Speed(); got != printing.SpeedNormal {
		t.Errorf("Speed() = %q, erwartet %q", got, printing.SpeedNormal)
	}
}
