package printing_test

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/torbenschinke/eventprint/app/printing"
)

// stubLpstat ersetzt lpstat durch ein Skript, das je nach Aufruf antwortet.
//
// QueryPrinter fragt dreimal: "-p" nach dem Gerät, "-a" nach der Annahme und
// "-W not-completed -o" nach dem Rückstau. Ein Skript, das alle drei bedient,
// ist der einzige Weg, den Weg von der Ausgabe bis zur Meldung im Ganzen zu
// prüfen – ohne CUPS und ohne Browser.
func stubLpstat(t *testing.T, printerOut string, printerExit int, acceptOut string) {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "lpstat")

	script := `#!/bin/sh
case "$1" in
  -p) printf '%s\n' ` + shellQuote(printerOut) + `; exit ` + strconv.Itoa(printerExit) + ` ;;
  -a) printf '%s\n' ` + shellQuote(acceptOut) + `; exit 0 ;;
  *)  exit 0 ;;
esac
`

	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("cannot write script: %v", err)
	}

	t.Cleanup(printing.SetLpstatExecutableForTest(path))
}

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// TestQueryPrinterReportsAnUnknownQueue ist der Befund, der uns mehrere
// Druckversuche gekostet hat: Eine Warteschlange, die es in CUPS nicht gibt,
// muss auffallen, statt jeden Auftrag ins Leere laufen zu lassen.
func TestQueryPrinterReportsAnUnknownQueue(t *testing.T) {
	// lpstat quittiert eine unbekannte Warteschlange mit einem Fehlercode.
	stubLpstat(t, "lpstat: Ungültiges Ziel »CZ01-e2e«", 1, "")

	status := printing.QueryPrinter(context.Background(), "CZ01-e2e")

	if status.Exists {
		t.Fatal("eine unbekannte Warteschlange gilt als eingerichtet")
	}

	if status.OK() {
		t.Fatal("eine unbekannte Warteschlange gilt als bereit")
	}

	want := "Die Warteschlange CZ01-e2e ist in CUPS nicht eingerichtet."
	if got := status.Problem(); got != want {
		t.Fatalf("Problem() = %q,\nerwartet         %q", got, want)
	}
}

// TestQueryPrinterReportsADisabledPrinter deckt den zweiten Fall ab, den die
// Betreuung sonst nur im Terminal sähe.
func TestQueryPrinterReportsADisabledPrinter(t *testing.T) {
	stubLpstat(t,
		"Drucker CZ01 ist deaktiviert seit Do 03 Sep 2026 11:41:39 CEST -\n\tOut of paper",
		0,
		"CZ01 akzeptiert Anfragen seit Do 03 Sep 2026 11:41:39 CEST")

	status := printing.QueryPrinter(context.Background(), "CZ01")

	if !status.Exists {
		t.Fatal("die vorhandene Warteschlange wurde nicht erkannt")
	}

	if status.Enabled {
		t.Fatal("ein angehaltener Drucker gilt als freigegeben")
	}

	if status.Problem() == "" {
		t.Fatal("zum angehaltenen Drucker fehlt die Erklärung im Klartext")
	}

	// Die Meldung des Geräts ist der eigentliche Hinweis für die Betreuung.
	if status.Message != "Out of paper" {
		t.Fatalf("Message = %q, erwartet die Gerätemeldung", status.Message)
	}
}

// TestQueryPrinterAcceptsAReadyPrinter ist die Gegenprobe: Ein bereiter
// Drucker darf keine Meldung erzeugen, sonst gewöhnt sich die Betreuung an
// einen Hinweis, der immer da steht.
func TestQueryPrinterAcceptsAReadyPrinter(t *testing.T) {
	stubLpstat(t,
		"Drucker CZ01 ist im Leerlauf.  Aktiviert seit Do 03 Sep 2026 11:41:39 CEST",
		0,
		"CZ01 akzeptiert Anfragen seit Do 03 Sep 2026 11:41:39 CEST")

	status := printing.QueryPrinter(context.Background(), "CZ01")

	if !status.OK() {
		t.Fatalf("ein bereiter Drucker gilt als nicht bereit: %q", status.Problem())
	}

	if status.Problem() != "" {
		t.Fatalf("Problem() = %q, erwartet leer", status.Problem())
	}
}
