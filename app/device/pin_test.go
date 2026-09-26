package device

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/worldiety/speclink/spec"
	"go.wdy.de/nago/application/permission"

	"github.com/torbenschinke/eventprint/requirements/fun/modus"
)

// TestSetPinValidation prüft die Form der PIN. Eine PIN wie 111111 errät
// jeder Gast beim zweiten Versuch; sie abzulehnen ist billiger als jede
// Sperre.
func TestSetPinValidation(t *testing.T) {
	f := newFixture(t)
	setPin := NewSetPin(f.mutex, f.settings)

	for _, pin := range []string{"", "12345", "1234567", "12a456", "12 456", "-12345", "000000", "999999", "１２３４５６"} {
		if err := setPin(owner(), pin); err == nil {
			t.Errorf("pin %q must be rejected", pin)
		}
	}

	s, _ := f.settings.Load()
	if s.Pin.Configured() {
		t.Fatal("rejected pins must not be stored")
	}

	var denied PermissionDeniedError
	if err := setPin(guest(), "135790"); !errors.As(err, &denied) {
		t.Fatalf("guest must not set a PIN, got %v", err)
	}

	if err := setPin(owner(), "135790"); err != nil {
		t.Fatal(err)
	}

	s, _ = f.settings.Load()
	if !s.Pin.Configured() || !s.Pin.Matches("135790") || s.Pin.Matches("135791") {
		t.Fatalf("stored hash must match exactly the PIN, got %+v", s.Pin)
	}

	spec.Verified(t, modus.RModusBetreuung)
}

// TestPinNotInClearText prüft, dass die PIN nie im Klartext auf der
// Speicherkarte liegt. Wer die Karte aus der Box zieht, soll sie nicht einfach
// ablesen können.
func TestPinNotInClearText(t *testing.T) {
	f := newFixture(t)

	const pin = "864209"
	if err := NewSetPin(f.mutex, f.settings)(owner(), pin); err != nil {
		t.Fatal(err)
	}

	buf, err := os.ReadFile(f.settings.path)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(buf), pin) {
		t.Fatalf("settings file contains the PIN in clear text:\n%s", buf)
	}

	// Jede Ableitung bekommt ein eigenes Salz, damit zwei Geräte mit
	// derselben PIN nicht denselben Hash tragen.
	a, err := HashPin(pin)
	if err != nil {
		t.Fatal(err)
	}

	b, err := HashPin(pin)
	if err != nil {
		t.Fatal(err)
	}

	if string(a.Salt) == string(b.Salt) || string(a.Hash) == string(b.Hash) {
		t.Fatal("hashing the same PIN twice must use different salts")
	}

	if (PinHash{}).Matches("") {
		t.Fatal("an unconfigured hash must never match")
	}

	spec.Verified(t, modus.RModusBetreuung)
}

// TestUnlock prüft Freischaltung, Ablauf und die Sperre nach Fehlversuchen
// mit einer Uhr, die der Test vorstellt.
func TestUnlock(t *testing.T) {
	f := newFixture(t)
	unlock := NewUnlock(f.settings, f.lock)

	// Ohne PIN gibt es nichts freizuschalten; der Notausstieg ist dann das
	// Ausschalten.
	if err := unlock(guest(), "135790"); !errors.Is(err, ErrNoPin) {
		t.Fatalf("no PIN configured: got %v, want ErrNoPin", err)
	}

	if err := NewSetPin(f.mutex, f.settings)(owner(), "135790"); err != nil {
		t.Fatal(err)
	}

	if f.lock.Unlocked() {
		t.Fatal("a new lock must start locked")
	}

	if err := unlock(guest(), "135790"); err != nil {
		t.Fatal(err)
	}

	if !f.lock.Unlocked() {
		t.Fatal("right PIN must unlock")
	}

	// Die Betreuung geht weg und vergisst abzuschließen. Nach Ablauf muss der
	// nächste Gast wieder vor einem gesperrten Gerät stehen.
	f.clock.Advance(UnlockTTL - time.Second)
	if !f.lock.Unlocked() {
		t.Fatal("unlock must hold until the TTL ends")
	}

	f.clock.Advance(time.Second)
	if f.lock.Unlocked() {
		t.Fatal("unlock must expire after UnlockTTL")
	}

	spec.Verified(t, modus.RModusBetreuung)
}

func TestUnlockThrottlesGuessing(t *testing.T) {
	f := newFixture(t)
	unlock := NewUnlock(f.settings, f.lock)

	if err := NewSetPin(f.mutex, f.settings)(owner(), "135790"); err != nil {
		t.Fatal(err)
	}

	for i := range PinMaxAttempts {
		if err := unlock(guest(), "000001"); !errors.Is(err, ErrPinWrong) {
			t.Fatalf("attempt %d: got %v, want ErrPinWrong", i+1, err)
		}
	}

	if f.lock.Unlocked() {
		t.Fatal("wrong PINs must not unlock")
	}

	// Jetzt ist gesperrt – auch die richtige PIN hilft nicht, sonst ließe
	// sich die Sperre durch Weiterraten umgehen.
	var locked PinLockedError
	if err := unlock(guest(), "135790"); !errors.As(err, &locked) {
		t.Fatalf("got %v, want PinLockedError", err)
	}

	first := locked.Retry
	if first <= 0 {
		t.Fatalf("retry must be positive, got %v", first)
	}

	if f.lock.Unlocked() {
		t.Fatal("a locked PIN must not unlock")
	}

	// Nach Ablauf der Sperre kostet jeder weitere Fehlversuch mehr Zeit.
	f.clock.Advance(first)
	if err := unlock(guest(), "000001"); !errors.Is(err, ErrPinWrong) {
		t.Fatalf("after block: got %v, want ErrPinWrong", err)
	}

	if err := unlock(guest(), "135790"); !errors.As(err, &locked) {
		t.Fatalf("got %v, want PinLockedError", err)
	}

	if locked.Retry <= first {
		t.Fatalf("retry must grow: first %v, now %v", first, locked.Retry)
	}

	// Die Sperre wächst nicht ins Unendliche; die Betreuung muss am selben
	// Abend noch hineinkommen.
	for range 20 {
		f.clock.Advance(locked.Retry)
		_ = unlock(guest(), "000001")
		if err := unlock(guest(), "135790"); !errors.As(err, &locked) {
			t.Fatalf("got %v, want PinLockedError", err)
		}
	}

	if locked.Retry > 15*time.Minute {
		t.Fatalf("retry must be capped at 15 minutes, got %v", locked.Retry)
	}

	// Die richtige PIN nach Ablauf schaltet frei und setzt den Zähler zurück.
	f.clock.Advance(locked.Retry)
	if err := unlock(guest(), "135790"); err != nil {
		t.Fatal(err)
	}

	if !f.lock.Unlocked() {
		t.Fatal("right PIN after the block must unlock")
	}

	if err := unlock(guest(), "000001"); !errors.Is(err, ErrPinWrong) {
		t.Fatalf("counter must reset after success, got %v", err)
	}

	spec.Verified(t, modus.RModusBetreuung)
}

// TestStopKiosk prüft den Notausstieg: Der Kiosk endet, und die Freischaltung
// endet mit ihm, damit das Gerät nicht offen im Heimbetrieb herumsteht.
func TestStopKiosk(t *testing.T) {
	f := newFixture(t)

	if err := NewSetPin(f.mutex, f.settings)(owner(), "135790"); err != nil {
		t.Fatal(err)
	}

	if _, err := f.startKiosk()(owner(), StartKioskCmd{Title: "Hochzeit"}); err != nil {
		t.Fatal(err)
	}

	stop := NewStopKiosk(f.kiosk, f.lock)

	var denied PermissionDeniedError
	if err := stop(guest()); !errors.As(err, &denied) {
		t.Fatalf("guest must not stop the kiosk, got %v", err)
	}

	if err := NewUnlock(f.settings, f.lock)(guest(), "135790"); err != nil {
		t.Fatal(err)
	}

	// Nach der PIN handelt die Betreuung mit den Rechten des Besitzers.
	operator := NewActor(RoleOperator, testGrants())
	if err := stop(operator); err != nil {
		t.Fatal(err)
	}

	if f.lock.Unlocked() {
		t.Fatal("stopping the kiosk must relock")
	}

	k, err := NewCurrentKiosk(f.kiosk)(owner())
	if err != nil {
		t.Fatal(err)
	}

	if k.Active() {
		t.Fatalf("kiosk must be over, got %+v", k)
	}

	// Die Feier selbst bleibt in der Liste; ihre Fotos gehören weiter zu ihr.
	s, _ := f.settings.Load()
	if len(s.Events) != 1 {
		t.Fatalf("stopping must keep the event, got %+v", s.Events)
	}

	spec.Verified(t, modus.RModusBetreuung)
}

// TestUnlockStaysThrottledAfterManyGuesses: Die Wartezeit wuchs durch
// Verdoppeln und lief nach gut dreißig Fehlversuchen über. Danach war sie
// negativ und das Raten unbegrenzt.
func TestUnlockStaysThrottledAfterManyGuesses(t *testing.T) {
	now := time.Unix(0, 0)
	lock := NewLock(func() time.Time { return now })
	hash, err := HashPin("135790")
	if err != nil {
		t.Fatal(err)
	}

	store := NewFileSettings(t.TempDir() + "/settings.json")
	if err := store.Save(Settings{Pin: hash}); err != nil {
		t.Fatal(err)
	}
	unlock := NewUnlock(store, lock)

	for i := range 100 {
		err := unlock(permission.SU(), "000000")
		if i >= PinMaxAttempts-1 {
			now = now.Add(16 * time.Minute) // die längste Sperre abwarten
		}

		if err == nil {
			t.Fatalf("Versuch %d: falsche PIN angenommen", i)
		}
	}

	if err := unlock(permission.SU(), "000000"); err != ErrPinWrong {
		t.Fatalf("erwartet ErrPinWrong, bekam %v", err)
	}

	// Unmittelbar danach muss die Sperre greifen.
	var locked PinLockedError
	if err := unlock(permission.SU(), "135790"); !errors.As(err, &locked) || locked.Retry <= 0 {
		t.Fatalf("nach hundert Fehlversuchen keine Sperre: %v", err)
	}

	spec.Verified(t, modus.RModusBetreuung)
}
