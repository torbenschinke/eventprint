package cfgphotobox

import (
	"errors"
	"testing"
	"time"
)

func testGate(t *testing.T, now *time.Time) *TapGate {
	t.Helper()

	g := NewTapGate()
	g.now = func() time.Time { return *now }

	return g
}

// TestGateOpensOnlyOnTheFifthTap haelt die Bedienung fest: vier Beruehrungen
// duerfen nichts tun, die fuenfte oeffnet.
func TestGateOpensOnlyOnTheFifthTap(t *testing.T) {
	clock := time.Date(2026, 9, 3, 20, 0, 0, 0, time.UTC)
	gate := testGate(t, &clock)

	for i := 1; i < TapsToConfigure; i++ {
		if gate.Tap() {
			t.Fatalf("das Tor ging schon bei Beruehrung %d auf", i)
		}

		clock = clock.Add(300 * time.Millisecond)
	}

	if !gate.Tap() {
		t.Fatalf("das Tor ging bei Beruehrung %d nicht auf", TapsToConfigure)
	}

	// Nach dem Oeffnen beginnt die Zaehlung von vorn, sonst oeffnete jede
	// weitere Beruehrung erneut.
	if gate.Tap() {
		t.Fatal("die naechste einzelne Beruehrung oeffnet erneut")
	}
}

// TestSlowTapsNeverOpenTheGate ist der eigentliche Zweck des Zeitfensters.
// Ueber einen Abend tippen Gaeste den QR-Code oft an; diese Beruehrungen
// duerfen sich nicht zum Zugang aufaddieren.
func TestSlowTapsNeverOpenTheGate(t *testing.T) {
	clock := time.Date(2026, 9, 3, 20, 0, 0, 0, time.UTC)
	gate := testGate(t, &clock)

	for range 50 {
		if gate.Tap() {
			t.Fatal("vereinzelte Beruehrungen haben das Tor geoeffnet")
		}

		clock = clock.Add(TapWindow + time.Second)
	}
}

// TestInterruptedSequenceStartsOver deckt den Grenzfall ab: vier zuegige
// Beruehrungen, eine lange Pause, dann eine weitere.
func TestInterruptedSequenceStartsOver(t *testing.T) {
	clock := time.Date(2026, 9, 3, 20, 0, 0, 0, time.UTC)
	gate := testGate(t, &clock)

	for range TapsToConfigure - 1 {
		gate.Tap()
		clock = clock.Add(200 * time.Millisecond)
	}

	clock = clock.Add(TapWindow + time.Second)

	if gate.Tap() {
		t.Fatal("nach der Pause zaehlte die alte Folge weiter")
	}
}

// TestConfigureIsOpenOnlyOnAFactoryNewBox beschreibt den Zustand eines
// fabrikneuen Geraets und den Moment, ab dem der Schutz greift.
func TestConfigureIsOpenOnlyOnAFactoryNewBox(t *testing.T) {
	clock := time.Date(2026, 9, 3, 20, 0, 0, 0, time.UTC)
	lock := testLock(t, &clock)

	var factoryNew PinHash

	// Fabrikneu: Wer davorsteht, darf die PIN vergeben. Anders ginge es nicht,
	// denn ein Geheimnis, das niemand kennt, sperrt auch den Aufbauenden aus.
	hash, err := lock.Configure("aufbau", "481902", factoryNew)
	if err != nil {
		t.Fatalf("auf einem fabrikneuen Geraet = %v, erwartet Erfolg", err)
	}

	if !hash.Configured() {
		t.Fatal("Configure lieferte keine brauchbare PIN")
	}

	// Wer die PIN gerade gesetzt hat, ist damit angemeldet.
	if !lock.Unlocked("aufbau") {
		t.Fatal("nach dem Festlegen ist die Sitzung nicht freigeschaltet")
	}

	// Ab jetzt greift der Schutz: Eine fremde Sitzung darf sie nicht ersetzen.
	if _, err := lock.Configure("gast", "000001", hash); !errors.Is(err, ErrPinNotUnlocked) {
		t.Fatalf("eine fremde Sitzung konnte die PIN ersetzen: %v", err)
	}

	// Die eigene, freigeschaltete Sitzung darf sie aendern.
	if _, err := lock.Configure("aufbau", "112358", hash); err != nil {
		t.Fatalf("die freigeschaltete Sitzung darf die PIN nicht aendern: %v", err)
	}

	// Nach Ablauf der Freischaltung ist auch sie wieder aussen vor.
	clock = clock.Add(PinSessionTTL + time.Minute)

	if _, err := lock.Configure("aufbau", "999998", hash); !errors.Is(err, ErrPinNotUnlocked) {
		t.Fatalf("nach Ablauf der Freischaltung = %v, erwartet ErrPinNotUnlocked", err)
	}
}

// TestConfigureRejectsAWeakPin stellt sicher, dass die Formregeln auch beim
// Festlegen gelten und nicht nur bei einer geprueften Eingabe.
func TestConfigureRejectsAWeakPin(t *testing.T) {
	clock := time.Date(2026, 9, 3, 20, 0, 0, 0, time.UTC)
	lock := testLock(t, &clock)

	for _, pin := range []string{"", "123", "abcdef", "111111"} {
		if _, err := lock.Configure("aufbau", pin, PinHash{}); err == nil {
			t.Fatalf("Configure nahm die schwache PIN %q an", pin)
		}
	}
}
