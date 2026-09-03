package printing

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestEnforceAbortPolicyPassesTheRightArguments hält den Aufruf fest, mit dem
// die Warteschlange umgestellt wird. Ein Tippfehler darin fiele sonst erst am
// Abend der Feier auf – durch einen doppelten Ausdruck.
func TestEnforceAbortPolicyPassesTheRightArguments(t *testing.T) {
	read := recordingLpadmin(t, 0)

	if err := EnforceAbortPolicy(context.Background(), "CZ01"); err != nil {
		t.Fatalf("EnforceAbortPolicy: %v", err)
	}

	want := "-p CZ01 -o printer-error-policy=abort-job"
	if got := read(); got != want {
		t.Fatalf("Aufruf = %q, erwartet %q", got, want)
	}
}

// TestEnforceAbortPolicyReportsFailure sichert ab, dass ein fehlendes Recht
// nicht verschluckt wird.
func TestEnforceAbortPolicyReportsFailure(t *testing.T) {
	recordingLpadmin(t, 1)

	err := EnforceAbortPolicy(context.Background(), "CZ01")
	if err == nil {
		t.Fatal("ein gescheitertes lpadmin muss gemeldet werden")
	}

	if !strings.Contains(err.Error(), "CZ01") {
		t.Errorf("Fehlermeldung nennt die Warteschlange nicht: %v", err)
	}
}

// TestEnforceAbortPolicyIgnoresEmptyQueue deckt den Testbetrieb ab, in dem gar
// keine Warteschlange eingerichtet ist.
func TestEnforceAbortPolicyIgnoresEmptyQueue(t *testing.T) {
	read := recordingLpadmin(t, 0)

	if err := EnforceAbortPolicy(context.Background(), ""); err != nil {
		t.Fatalf("EnforceAbortPolicy: %v", err)
	}

	if got := read(); got != "" {
		t.Fatalf("ohne Warteschlange wurde %q aufgerufen", got)
	}
}

// TestPolicyGuardRunsOncePerQueue ist die eigentliche Zusage des Wächters.
//
// Die Einstellung überlebt in CUPS jeden Neustart. Sie vor jedem Auftrag
// erneut zu setzen wäre nutzloser Aufwand auf dem Weg zum Drucker – und der
// Weg ist auf einer Feier der einzige, der zählt.
func TestPolicyGuardRunsOncePerQueue(t *testing.T) {
	count := countingLpadmin(t)

	var guard policyGuard

	guard.ensure(context.Background(), "CZ01")
	guard.ensure(context.Background(), "CZ01")
	guard.ensure(context.Background(), "CZ01")

	if got := count(); got != 1 {
		t.Fatalf("lpadmin lief %dx für dieselbe Warteschlange, erwartet 1x", got)
	}

	// Ein Wechsel der Warteschlange im laufenden Betrieb muss dagegen greifen.
	guard.ensure(context.Background(), "CZ02")

	if got := count(); got != 2 {
		t.Fatalf("lpadmin lief %dx, erwartet 2x – die neue Warteschlange wurde übergangen", got)
	}
}

// recordingLpadmin ersetzt lpadmin durch ein Skript, das seine Argumente
// festhält und mit exitCode endet.
func recordingLpadmin(t *testing.T, exitCode int) func() string {
	t.Helper()

	dir := t.TempDir()
	log := filepath.Join(dir, "args")

	script := "#!/bin/sh\nprintf '%s' \"$*\" > " + log + "\nexit " + strconv.Itoa(exitCode) + "\n"
	writeScript(t, dir, script)

	return func() string {
		buf, err := os.ReadFile(log)
		if err != nil {
			return ""
		}

		return strings.TrimSpace(string(buf))
	}
}

// countingLpadmin zählt die Aufrufe.
func countingLpadmin(t *testing.T) func() int {
	t.Helper()

	dir := t.TempDir()
	log := filepath.Join(dir, "calls")

	writeScript(t, dir, "#!/bin/sh\necho x >> "+log+"\n")

	return func() int {
		buf, err := os.ReadFile(log)
		if err != nil {
			return 0
		}

		return strings.Count(string(buf), "x")
	}
}

func writeScript(t *testing.T, dir, body string) {
	t.Helper()

	path := filepath.Join(dir, "lpadmin")
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("cannot write script: %v", err)
	}

	old := lpadminExecutable
	lpadminExecutable = path
	t.Cleanup(func() { lpadminExecutable = old })
}
