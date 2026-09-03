package cfgphotobox

import (
	"errors"
	"testing"
	"time"
)

// TestValidPinRejectsWeakInput haelt die Formregeln fest. Eine PIN aus einer
// einzigen Ziffer waere auf einem Tastenfeld die erste Eingabe jedes Gastes.
func TestValidPinRejectsWeakInput(t *testing.T) {
	tests := []struct {
		name string
		pin  string
		ok   bool
	}{
		{name: "gute PIN", pin: "481902", ok: true},
		{name: "zu kurz", pin: "4819"},
		{name: "zu lang", pin: "4819021"},
		{name: "Buchstaben", pin: "48a902"},
		{name: "leer", pin: ""},
		{name: "eine einzige Ziffer", pin: "777777"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidPin(tt.pin)
			if tt.ok && err != nil {
				t.Fatalf("ValidPin(%q) = %v, erwartet keinen Fehler", tt.pin, err)
			}

			if !tt.ok && err == nil {
				t.Fatalf("ValidPin(%q) hat keinen Fehler gemeldet", tt.pin)
			}
		})
	}
}

// TestPinHashDoesNotStoreTheSecret ist die Kernzusage der Ablage.
func TestPinHashDoesNotStoreTheSecret(t *testing.T) {
	const pin = "481902"

	h, err := HashPin(pin)
	if err != nil {
		t.Fatalf("HashPin: %v", err)
	}

	if !h.Configured() {
		t.Fatal("die abgeleitete PIN gilt als nicht eingerichtet")
	}

	if string(h.Hash) == pin || string(h.Salt) == pin {
		t.Fatal("die PIN steht im Klartext in der Ablage")
	}

	if !h.Matches(pin) {
		t.Fatal("die richtige PIN wird nicht erkannt")
	}

	if h.Matches("481903") {
		t.Fatal("eine falsche PIN wird angenommen")
	}

	// Zwei Ableitungen derselben PIN muessen sich unterscheiden, sonst verriete
	// ein Vergleich zweier Fotoboxen die gleiche PIN.
	h2, err := HashPin(pin)
	if err != nil {
		t.Fatalf("HashPin: %v", err)
	}

	if string(h.Hash) == string(h2.Hash) {
		t.Fatal("zwei Ableitungen derselben PIN sind gleich; das Salz wirkt nicht")
	}
}

// TestUnconfiguredPinNeverMatches deckt den gefaehrlichsten Zustand ab: eine
// leere Ablage darf nicht jede Eingabe durchlassen.
func TestUnconfiguredPinNeverMatches(t *testing.T) {
	var empty PinHash

	if empty.Configured() {
		t.Fatal("eine leere Ablage gilt als eingerichtet")
	}

	for _, pin := range []string{"", "000000", "481902"} {
		if empty.Matches(pin) {
			t.Fatalf("die leere Ablage nimmt %q an", pin)
		}
	}

	lock := NewPinLock()
	if err := lock.Verify("s1", "000000", empty); !errors.Is(err, ErrNoPin) {
		t.Fatalf("Verify ohne eingerichtete PIN = %v, erwartet ErrNoPin", err)
	}

	if lock.Unlocked("s1") {
		t.Fatal("ohne eingerichtete PIN wurde freigeschaltet")
	}
}

func testLock(t *testing.T, now *time.Time) *PinLock {
	t.Helper()

	l := NewPinLock()
	l.now = func() time.Time { return *now }

	return l
}

// TestWrongPinBlocksAfterThreeTries ist der Schutz gegen das Durchprobieren am
// Geraet. Ohne ihn waeren sechs Stellen an einem Tastenfeld kein Hindernis.
func TestWrongPinBlocksAfterThreeTries(t *testing.T) {
	clock := time.Date(2026, 9, 3, 20, 0, 0, 0, time.UTC)
	lock := testLock(t, &clock)

	hash, err := HashPin("481902")
	if err != nil {
		t.Fatalf("HashPin: %v", err)
	}

	for i := range PinMaxAttempts {
		if err := lock.Verify("s1", "000001", hash); !errors.Is(err, ErrPinWrong) {
			t.Fatalf("Versuch %d = %v, erwartet ErrPinWrong", i+1, err)
		}
	}

	// Jetzt muss auch die RICHTIGE PIN abgewiesen werden, sonst waere die
	// Sperre wirkungslos.
	var locked ErrPinLocked
	if err := lock.Verify("s1", "481902", hash); !errors.As(err, &locked) {
		t.Fatalf("nach %d Fehlversuchen = %v, erwartet eine Sperre", PinMaxAttempts, err)
	}

	if locked.Retry <= 0 {
		t.Fatalf("die Sperre nennt keine Wartezeit: %v", locked.Retry)
	}

	// Nach Ablauf der Wartezeit kommt der Betreuer wieder hinein. Eine
	// endgueltige Sperre waere eine Einladung, die Box lahmzulegen.
	clock = clock.Add(locked.Retry + time.Second)

	if err := lock.Verify("s1", "481902", hash); err != nil {
		t.Fatalf("nach Ablauf der Sperre = %v, erwartet Erfolg", err)
	}

	if !lock.Unlocked("s1") {
		t.Fatal("nach richtiger PIN ist die Sitzung nicht freigeschaltet")
	}
}

// TestBlockGrowsAndSurvivesANewSession haelt fest, dass der Zaehler
// anwendungsweit gilt. Sonst umginge man die Sperre durch ein neues Fenster.
func TestBlockGrowsAndSurvivesANewSession(t *testing.T) {
	clock := time.Date(2026, 9, 3, 20, 0, 0, 0, time.UTC)
	lock := testLock(t, &clock)

	hash, err := HashPin("481902")
	if err != nil {
		t.Fatalf("HashPin: %v", err)
	}

	for range PinMaxAttempts {
		_ = lock.Verify("s1", "000001", hash)
	}

	var first ErrPinLocked
	if err := lock.Verify("s2", "000001", hash); !errors.As(err, &first) {
		t.Fatalf("eine neue Sitzung umgeht die Sperre: %v", err)
	}

	// Weiter raten verlaengert die Wartezeit.
	clock = clock.Add(first.Retry + time.Second)
	_ = lock.Verify("s1", "000001", hash)

	var second ErrPinLocked
	if err := lock.Verify("s1", "000001", hash); !errors.As(err, &second) {
		t.Fatalf("nach erneutem Fehlversuch keine Sperre: %v", err)
	}

	if second.Retry <= first.Retry {
		t.Fatalf("die Sperre waechst nicht: erst %v, dann %v", first.Retry, second.Retry)
	}
}

// TestUnlockExpires stellt sicher, dass eine vergessene Sitzung nicht den Abend
// offen laesst. Die Box steht unbeaufsichtigt.
func TestUnlockExpires(t *testing.T) {
	clock := time.Date(2026, 9, 3, 20, 0, 0, 0, time.UTC)
	lock := testLock(t, &clock)

	hash, err := HashPin("481902")
	if err != nil {
		t.Fatalf("HashPin: %v", err)
	}

	if err := lock.Verify("s1", "481902", hash); err != nil {
		t.Fatalf("Verify: %v", err)
	}

	clock = clock.Add(PinSessionTTL - time.Minute)
	if !lock.Unlocked("s1") {
		t.Fatal("die Freischaltung endete zu frueh")
	}

	clock = clock.Add(2 * time.Minute)
	if lock.Unlocked("s1") {
		t.Fatal("die Freischaltung gilt nach Ablauf weiter")
	}
}

// TestUnlockIsPerSession verhindert den schlimmsten Denkfehler: Der Betreuer
// meldet sich an, und jeder Gast am selben Geraet ist mit angemeldet.
func TestUnlockIsPerSession(t *testing.T) {
	clock := time.Date(2026, 9, 3, 20, 0, 0, 0, time.UTC)
	lock := testLock(t, &clock)

	hash, err := HashPin("481902")
	if err != nil {
		t.Fatalf("HashPin: %v", err)
	}

	if err := lock.Verify("betreuer", "481902", hash); err != nil {
		t.Fatalf("Verify: %v", err)
	}

	if lock.Unlocked("gast") {
		t.Fatal("eine fremde Sitzung ist mit freigeschaltet")
	}

	lock.Lock("betreuer")
	if lock.Unlocked("betreuer") {
		t.Fatal("Lock hat die Freischaltung nicht zurueckgenommen")
	}
}
