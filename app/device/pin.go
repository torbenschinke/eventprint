package device

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"
)

// PinLength ist die Länge der Betreuer-PIN.
//
// Sechs Stellen sind ein Kompromiss: Auf einem Tastenfeld im Halbdunkel ist
// alles darüber lästig, und darunter wird das Raten selbst mit der Sperre zu
// billig.
const PinLength = 6

// PinMaxAttempts ist die Zahl der Fehlversuche vor der ersten Sperre.
const PinMaxAttempts = 3

// UnlockTTL ist die Gültigkeit einer Freischaltung.
//
// Die Box steht unbeaufsichtigt. Bleibt die Betreuung freigeschaltet und geht
// weg, hätte der nächste Gast alle Rechte.
const UnlockTTL = 10 * time.Minute

// PinHash ist die PIN in abgeleiteter Form.
type PinHash struct {
	Salt []byte `json:"salt,omitempty"`
	Hash []byte `json:"hash,omitempty"`
}

// Configured meldet, ob eine PIN festgelegt ist.
func (h PinHash) Configured() bool { return len(h.Salt) > 0 && len(h.Hash) > 0 }

// Matches prüft eine eingegebene PIN.
func (h PinHash) Matches(pin string) bool {
	if !h.Configured() {
		return false
	}

	return bytes.Equal(derive(pin, h.Salt), h.Hash)
}

// HashPin leitet eine neue PIN ab.
func HashPin(pin string) (PinHash, error) {
	if err := ValidPin(pin); err != nil {
		return PinHash{}, err
	}

	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return PinHash{}, fmt.Errorf("PIN kann nicht abgeleitet werden: %w", err)
	}

	return PinHash{Salt: salt, Hash: derive(pin, salt)}, nil
}

// derive ist Argon2id mit bescheidenen Parametern: Ein Raspberry Pi soll die
// Eingabe in einem Bruchteil einer Sekunde prüfen, und die Sperre nach
// Fehlversuchen trägt die eigentliche Last gegen Raten.
func derive(pin string, salt []byte) []byte {
	return argon2.IDKey([]byte(pin), salt, 1, 19*1024, 1, 32)
}

// ValidPin prüft die Form einer neuen PIN.
func ValidPin(pin string) error {
	if len(pin) != PinLength {
		return fmt.Errorf("die PIN muss %d Stellen haben", PinLength)
	}

	for _, r := range pin {
		if r < '0' || r > '9' {
			return errors.New("die PIN darf nur Ziffern enthalten")
		}
	}

	if strings.Count(pin, pin[:1]) == len(pin) {
		return errors.New("die PIN darf nicht aus einer einzigen Ziffer bestehen")
	}

	return nil
}

// ErrPinWrong meldet eine falsche PIN.
var ErrPinWrong = errors.New("falsche PIN")

// ErrNoPin meldet, dass keine PIN festgelegt ist.
var ErrNoPin = errors.New("es ist keine PIN festgelegt")

// PinLockedError meldet die Sperre nach Fehlversuchen.
type PinLockedError struct {
	Retry time.Duration
}

func (e PinLockedError) Error() string {
	return fmt.Sprintf("zu viele Fehlversuche, noch %s gesperrt", e.Retry.Round(time.Second))
}

// Lock verwaltet die Freischaltung der Betreuung und bremst das Raten aus.
//
// Der Zustand liegt nur im Speicher. Ein Neustart sperrt jede Freischaltung
// wieder zu – nach einem Neustart weiß niemand mehr, wer vor dem Bildschirm
// steht.
type Lock struct {
	mu         sync.Mutex
	until      time.Time
	failures   int
	blockedTil time.Time
	now        func() time.Time
}

// NewLock erzeugt eine gesperrte Freischaltung.
func NewLock(now func() time.Time) *Lock {
	if now == nil {
		now = time.Now
	}

	return &Lock{now: now}
}

// Unlocked meldet, ob die Betreuung gerade freigeschaltet ist.
func (l *Lock) Unlocked() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.now().Before(l.until)
}

// Relock beendet eine Freischaltung sofort.
func (l *Lock) Relock() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.until = time.Time{}
}

func (l *Lock) verify(pin string, hash PinHash) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	if now.Before(l.blockedTil) {
		return PinLockedError{Retry: l.blockedTil.Sub(now)}
	}

	if !hash.Configured() {
		return ErrNoPin
	}

	if !hash.Matches(pin) {
		l.failures++
		if l.failures >= PinMaxAttempts {
			// Der Exponent wird vor dem Schieben begrenzt. Danach begrenzt,
			// liefe die Multiplikation nach gut dreißig Fehlversuchen über,
			// die Wartezeit würde negativ – und das Raten wäre unbegrenzt.
			shift := min(l.failures-PinMaxAttempts, 6)
			d := time.Duration(1<<shift) * 15 * time.Second
			l.blockedTil = now.Add(min(d, 15*time.Minute))
		}

		return ErrPinWrong
	}

	l.failures = 0
	l.blockedTil = time.Time{}
	l.until = now.Add(UnlockTTL)

	return nil
}
