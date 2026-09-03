package cfgphotobox

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.wdy.de/nago/application/user"
)

// PinLength ist die Länge der Betreuer-PIN.
//
// Sechs Stellen sind ein Kompromiss: Auf einem Tastenfeld im Halbdunkel ist
// alles darüber lästig, und darunter wird das Raten selbst mit der Sperre
// unten zu billig. Eine sechsstellige PIN hat eine Million Möglichkeiten; mit
// der wachsenden Sperre nach drei Fehlversuchen dauert das Durchprobieren
// länger als jede Feier.
const PinLength = 6

// PinMaxAttempts ist die Zahl der Fehlversuche vor der ersten Sperre.
const PinMaxAttempts = 3

// PinSessionTTL ist die Gültigkeit einer Freischaltung.
//
// Die Fotobox steht unbeaufsichtigt. Bleibt der Betreuer angemeldet und geht
// weg, hätte der nächste Gast alle Rechte. Eine halbe Stunde reicht für das
// Einrichten und ist kurz genug, dass eine vergessene Sitzung nicht den Abend
// offen lässt.
const PinSessionTTL = 30 * time.Minute

// ErrPinLocked meldet, dass wegen Fehlversuchen gerade keine Eingabe möglich ist.
type ErrPinLocked struct {
	// Retry ist die verbleibende Wartezeit.
	Retry time.Duration
}

func (e ErrPinLocked) Error() string {
	return fmt.Sprintf("PIN-Eingabe gesperrt, noch %s", e.Retry.Round(time.Second))
}

// ErrPinWrong meldet eine falsche PIN.
var ErrPinWrong = errors.New("falsche PIN")

// ErrNoPin meldet, dass gar keine PIN eingerichtet ist.
var ErrNoPin = errors.New("keine PIN eingerichtet")

// PinLock verwaltet die Freischaltungen und bremst das Raten aus.
//
// Der Zustand liegt bewusst nur im Speicher. Ein Neustart der Anwendung sperrt
// damit jede offene Sitzung wieder zu – das ist die gewünschte Richtung, denn
// nach einem Neustart weiß niemand mehr, wer vor dem Bildschirm steht.
//
// Der Fehlerzähler gilt anwendungsweit und nicht je Sitzung. Sonst umginge man
// die Sperre, indem man die Seite in einem privaten Fenster neu öffnet.
type PinLock struct {
	mu         sync.Mutex
	unlocked   map[string]time.Time
	failures   int
	blockedTil time.Time

	// now ist auswechselbar, damit die Sperre prüfbar ist, ohne im Test
	// tatsächlich Minuten zu warten.
	now func() time.Time
}

// NewPinLock erstellt eine leere Verwaltung.
func NewPinLock() *PinLock {
	return &PinLock{unlocked: map[string]time.Time{}, now: time.Now}
}

// pinBlockFor ist die Wartezeit nach n Fehlversuchen.
//
// Die Zeit wächst, statt endgültig zu sperren: Ein endgültiger Riegel wäre eine
// Einladung, die Fotobox durch dreimal falsches Tippen lahmzulegen. So kommt
// der Betreuer nach kurzem Warten wieder hinein, ein Ratender aber nicht durch.
func pinBlockFor(failures int) time.Duration {
	if failures < PinMaxAttempts {
		return 0
	}

	d := time.Duration(1<<(failures-PinMaxAttempts)) * 15 * time.Second
	if d > 15*time.Minute {
		d = 15 * time.Minute
	}

	return d
}

// Verify prüft die PIN einer Sitzung und schaltet sie bei Erfolg frei.
func (l *PinLock) Verify(sessionID string, pin string, hash PinHash) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()

	if now.Before(l.blockedTil) {
		return ErrPinLocked{Retry: l.blockedTil.Sub(now)}
	}

	if !hash.Configured() {
		return ErrNoPin
	}

	if !hash.Matches(pin) {
		l.failures++
		if d := pinBlockFor(l.failures); d > 0 {
			l.blockedTil = now.Add(d)
		}

		return ErrPinWrong
	}

	// Ein Treffer räumt den Zähler ab, sonst summierten sich Vertipper über
	// den Abend zu einer Sperre für den Betreuer.
	l.failures = 0
	l.blockedTil = time.Time{}
	l.unlocked[sessionID] = now.Add(PinSessionTTL)

	return nil
}

// Unlocked meldet, ob die Sitzung derzeit freigeschaltet ist.
func (l *PinLock) Unlocked(sessionID string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	until, ok := l.unlocked[sessionID]
	if !ok {
		return false
	}

	if !l.now().Before(until) {
		delete(l.unlocked, sessionID)
		return false
	}

	return true
}

// Lock nimmt eine Freischaltung zurück.
func (l *PinLock) Lock(sessionID string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.unlocked, sessionID)
}

// ErrPinNotUnlocked meldet den Versuch, eine bestehende PIN ohne Freischaltung
// zu überschreiben.
var ErrPinNotUnlocked = errors.New("zum Ändern der PIN erst mit der bisherigen anmelden")

// Configure legt die PIN fest und schaltet die Sitzung frei.
//
// Ein fabrikneues Gerät hat keine PIN. In diesem Zustand darf sie jeder
// vergeben, der davorsteht – das ist unvermeidlich, denn ein Geheimnis, das
// niemand kennt, sperrt auch den Aufbauenden aus. Der Schutz beginnt mit der
// ersten Vergabe: Danach kommt nur noch hinein, wer die bisherige PIN kennt.
//
// Daraus folgt eine Betriebsregel, die in der Anleitung stehen muss: Die PIN
// gehört beim Aufbau vergeben, nicht wenn die ersten Gäste schon da sind.
func (l *PinLock) Configure(sessionID string, pin string, current PinHash) (PinHash, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if current.Configured() {
		until, ok := l.unlocked[sessionID]
		if !ok || !l.now().Before(until) {
			return PinHash{}, ErrPinNotUnlocked
		}
	}

	next, err := HashPin(pin)
	if err != nil {
		return PinHash{}, err
	}

	// Wer die PIN gerade gesetzt hat, ist damit angemeldet. Ihn zusätzlich
	// tippen zu lassen, was er eben zweimal eingegeben hat, wäre Schikane.
	l.unlocked[sessionID] = l.now().Add(PinSessionTTL)
	l.failures = 0
	l.blockedTil = time.Time{}

	return next, nil
}

// PinHash ist die gespeicherte Form der PIN.
//
// Die PIN steht nie im Klartext in den Einstellungen. Sie erscheint dort als
// Argon2-Ableitung, damit ein Blick in die Datenablage sie nicht verrät.
type PinHash struct {
	Salt []byte
	Hash []byte
}

// Configured meldet, ob überhaupt eine PIN hinterlegt ist.
func (h PinHash) Configured() bool { return len(h.Salt) > 0 && len(h.Hash) > 0 }

// Matches prüft eine eingegebene PIN.
func (h PinHash) Matches(pin string) bool {
	if !h.Configured() {
		return false
	}

	err := user.Password(pin).CompareHashAndPassword(user.Argon2IdMin, h.Salt, h.Hash)

	return err == nil
}

// HashPin bildet die speicherbare Form einer PIN.
//
// user.Password.Validate wird bewusst nicht gerufen: Es verlangt ein langes
// Passwort und würde jede PIN ablehnen. Die Ableitung selbst ist davon
// unberührt und dieselbe wie bei einem Passwort.
func HashPin(pin string) (PinHash, error) {
	if err := ValidPin(pin); err != nil {
		return PinHash{}, err
	}

	salt, hash, err := user.Password(pin).Hash(user.Argon2IdMin)
	if err != nil {
		return PinHash{}, fmt.Errorf("PIN kann nicht abgeleitet werden: %w", err)
	}

	return PinHash{Salt: salt, Hash: hash}, nil
}

// ValidPin prüft eine PIN auf Form und offensichtliche Schwäche.
func ValidPin(pin string) error {
	if len(pin) != PinLength {
		return fmt.Errorf("die PIN muss %d Stellen haben", PinLength)
	}

	for _, r := range pin {
		if r < '0' || r > '9' {
			return errors.New("die PIN darf nur Ziffern enthalten")
		}
	}

	// Eine PIN aus einer einzigen Ziffer errät jeder Gast im ersten Versuch.
	if strings.Count(pin, string(pin[0])) == len(pin) {
		return errors.New("die PIN darf nicht aus einer einzigen Ziffer bestehen")
	}

	return nil
}
