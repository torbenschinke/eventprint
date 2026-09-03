package wifi_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/worldiety/speclink/spec"
	"go.wdy.de/nago/application/user"

	"github.com/torbenschinke/eventprint/app/wifi"
	"github.com/torbenschinke/eventprint/requirements/fun/netz"
)

// stubNmcli ersetzt nmcli durch ein Skript mit vorgegebener Ausgabe und
// vorgegebenem Rueckgabewert.
//
// Anders liesse sich der Verbindungsaufbau nicht pruefen, ohne die
// Funkverbindung des Rechners anzufassen, auf dem der Test laeuft.
func stubNmcli(t *testing.T, output string, exit int) {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "nmcli")

	script := "#!/bin/sh\ncat <<'OUT'\n" + output + "\nOUT\nexit " + itoa(exit) + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("cannot write stub: %v", err)
	}

	t.Cleanup(wifi.SetNmcliExecutableForTest(path))
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}

	return "1"
}

// TestWrongPasswordIsToldApartFromOtherFailures ist der Unterschied, auf den
// es vor Ort ankommt: neu eintippen oder naeher ans Funknetz.
func TestWrongPasswordIsToldApartFromOtherFailures(t *testing.T) {
	stubNmcli(t, "Error: Connection activation failed: (7) Secrets were required, but not provided.", 1)

	connect := wifi.NewConnect()

	err := connect(user.SU(), context.Background(), "Schinke", "falsch")

	var wrong wifi.WrongPasswordError
	if !errors.As(err, &wrong) {
		t.Fatalf("err = %v, erwartet WrongPasswordError", err)
	}

	if wrong.SSID != "Schinke" {
		t.Fatalf("SSID in der Meldung = %q", wrong.SSID)
	}

	spec.Verified(t, netz.RNetzVerbinden)
}

// TestOtherFailuresAreNotBlamedOnThePassword ist die Gegenprobe. Wer wegen zu
// schwachem Empfang scheitert, tippt sonst endlos ein richtiges Kennwort neu.
func TestOtherFailuresAreNotBlamedOnThePassword(t *testing.T) {
	stubNmcli(t, "Error: No network with SSID 'Nachbar' found.", 1)

	connect := wifi.NewConnect()

	err := connect(user.SU(), context.Background(), "Nachbar", "egal")

	var wrong wifi.WrongPasswordError
	if errors.As(err, &wrong) {
		t.Fatalf("ein fremder Fehler wurde dem Kennwort angelastet: %v", err)
	}

	if err == nil {
		t.Fatal("der gescheiterte Verbindungsaufbau wurde nicht gemeldet")
	}
}

// TestConnectSucceeds deckt den Normalfall ab.
func TestConnectSucceeds(t *testing.T) {
	stubNmcli(t, "Device 'wlan0' successfully activated with 'abc'.", 0)

	if err := wifi.NewConnect()(user.SU(), context.Background(), "Schinke", "geheim"); err != nil {
		t.Fatalf("err = %v, erwartet Erfolg", err)
	}
}

// TestConnectRefusesAnEmptySsid: Ein Verbindungsversuch ohne Netzname wuerde
// nmcli mit einer unverstaendlichen Meldung quittieren.
func TestConnectRefusesAnEmptySsid(t *testing.T) {
	if err := wifi.NewConnect()(user.SU(), context.Background(), "   ", ""); err == nil {
		t.Fatal("ein leerer Netzname wurde angenommen")
	}
}

// TestGuestMayNotChangeTheNetwork haelt fest, dass die Berechtigung greift.
// Ein Gast, der das Funknetz wechselt, nimmt der Fotobox die Verbindung.
func TestGuestMayNotChangeTheNetwork(t *testing.T) {
	stubNmcli(t, "", 0)

	if err := wifi.NewConnect()(guest{}, context.Background(), "Schinke", ""); err == nil {
		t.Fatal("ein Gast durfte das Funknetz wechseln")
	}

	if _, err := wifi.NewScan()(guest{}, context.Background()); err == nil {
		t.Fatal("ein Gast durfte nach Funknetzen suchen")
	}

	if _, err := wifi.NewCurrent()(guest{}, context.Background()); err == nil {
		t.Fatal("ein Gast durfte die Verbindung abfragen")
	}

	spec.Verified(t, netz.RNetzBetreuung)
}
