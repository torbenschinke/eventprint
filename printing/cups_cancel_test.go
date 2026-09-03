package printing_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/worldiety/speclink/spec"

	"github.com/torbenschinke/eventprint/printing"
	"github.com/torbenschinke/eventprint/requirements/fun/druck"
)

// TestAwaitJobCancelsAbandonedJob sichert die Ursache des Doppeldrucks ab.
//
// Gibt die Fotobox einen Auftrag nach Ablauf der Frist auf, muss sie ihn im
// Druckdienst zurücknehmen. Unterbleibt das, bleibt der Auftrag dort gültig
// und wird ausgegeben, sobald der Drucker wieder erreichbar ist – unabhängig
// davon, dass die Fotobox ihn längst als fehlgeschlagen führt.
func TestAwaitJobCancelsAbandonedJob(t *testing.T) {
	record := recordingCancel(t)

	outcome := printing.AwaitJob(context.Background(), "NichtVorhanden", "CZ01-31", 10*time.Millisecond, time.Millisecond)

	if outcome.Reason != "timeout" {
		t.Fatalf("Reason = %q, erwartet timeout", outcome.Reason)
	}

	if got := record(); got != "-x CZ01-31" {
		t.Fatalf("Storno = %q, erwartet '-x CZ01-31'", got)
	}

	spec.Verified(t, druck.RDruckKeinNachdruck)
}

// TestAwaitJobCancelsOnShutdown deckt den zweiten Weg ab, auf dem ein Auftrag
// zurückbleiben kann: Die Fotobox wird beendet, während CUPS noch arbeitet.
func TestAwaitJobCancelsOnShutdown(t *testing.T) {
	record := recordingCancel(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	outcome := printing.AwaitJob(ctx, "NichtVorhanden", "CZ01-32", time.Minute, time.Millisecond)

	if outcome.Reason != "canceled" {
		t.Fatalf("Reason = %q, erwartet canceled", outcome.Reason)
	}

	// Der Kontext ist bereits abgebrochen. Das Storno muss trotzdem laufen,
	// sonst wäre jedes Herunterfahren eine Quelle für Nachdrucke.
	if got := record(); got != "-x CZ01-32" {
		t.Fatalf("Storno = %q, erwartet '-x CZ01-32'", got)
	}
}

// recordingCancel ersetzt das Storno-Kommando durch ein Skript, das seine
// Argumente festhält, und liefert den Lesezugriff darauf.
func recordingCancel(t *testing.T) func() string {
	t.Helper()

	dir := t.TempDir()
	log := filepath.Join(dir, "args")
	script := filepath.Join(dir, "cancel")

	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf '%s' \"$*\" > "+log+"\n"), 0o755); err != nil {
		t.Fatalf("cannot write script: %v", err)
	}

	t.Cleanup(printing.SetCancelExecutableForTest(script))

	return func() string {
		buf, err := os.ReadFile(log)
		if err != nil {
			return ""
		}

		return strings.TrimSpace(string(buf))
	}
}

func TestParsePendingJobs(t *testing.T) {
	// Echte Ausgabe von "lpstat -W not-completed -o CZ01".
	const out = `CZ01-31                 tschinke        711680   Mo 31 Aug 2026 20:42:50 CEST
CZ01-32                 tschinke        758784   Mo 31 Aug 2026 20:44:11 CEST
`

	got := printing.ParsePendingJobs(out)

	if len(got) != 2 || got[0] != "CZ01-31" || got[1] != "CZ01-32" {
		t.Fatalf("ParsePendingJobs = %v", got)
	}

	if got := printing.ParsePendingJobs(""); len(got) != 0 {
		t.Fatalf("leere Ausgabe = %v, erwartet keine Aufträge", got)
	}
}
