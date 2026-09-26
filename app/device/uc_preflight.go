package device

import (
	"context"

	"go.wdy.de/nago/application/permission"
)

// Level ist die Schwere eines Prüfergebnisses.
type Level int

const (
	LevelOK Level = iota
	LevelWarning
	LevelError
)

// Check ist ein einzelnes Prüfergebnis.
type Check struct {
	Title  string
	Detail string
	Level  Level
}

// Report ist das Ergebnis der Vorabprüfung.
type Report struct {
	Checks []Check
}

// Blocking meldet, ob etwas den Kiosk unbrauchbar machen würde.
func (r Report) Blocking() bool {
	for _, c := range r.Checks {
		if c.Level == LevelError {
			return true
		}
	}

	return false
}

// Probe prüft einen Aspekt des Geräts. Die Verdrahtung liefert sie, weil nur
// sie Drucker, Upload-Dienst und Kamera kennt.
type Probe func(ctx context.Context, s Settings) Check

// Preflight prüft vor einer Feier, ob das Gerät bereit ist.
//
// Ein Fehler, der beim Aufbau auffällt, kostet eine Minute. Derselbe Fehler
// mitten auf der Feier kostet den Abend – dann steht die Box mit einem
// Gast davor, und niemand weiß, wo die Einstellungen sind.
type Preflight func(subject permission.Auditable, ctx context.Context) (Report, error)

// NewPreflight erzeugt den [Preflight] Anwendungsfall.
func NewPreflight(store SettingsStore, probes []Probe) Preflight {
	return func(subject permission.Auditable, ctx context.Context) (Report, error) {
		if err := subject.Audit(PermPreflight); err != nil {
			return Report{}, err
		}

		s, err := store.Load()
		if err != nil {
			return Report{}, err
		}

		s = s.Normalized()

		var r Report
		for _, probe := range probes {
			r.Checks = append(r.Checks, probe(ctx, s))
		}

		if s.Pin.Configured() {
			r.Checks = append(r.Checks, Check{Title: "Betreuer-PIN festgelegt", Detail: "Für den Notausstieg ohne Neustart", Level: LevelOK})
		} else {
			r.Checks = append(r.Checks, Check{Title: "Keine Betreuer-PIN", Detail: "Den Kiosk beendet dann nur ein Neustart", Level: LevelWarning})
		}

		return r, nil
	}
}
