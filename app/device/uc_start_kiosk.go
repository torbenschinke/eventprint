package device

import (
	"errors"
	"strings"
	"sync"
	"time"

	"go.wdy.de/nago/application/permission"

	"github.com/torbenschinke/eventprint/app/photo"
)

// StartKioskCmd beschreibt die Feier, die beginnt.
type StartKioskCmd struct {
	Title string
}

// StartKiosk versetzt das Gerät in den Kiosk-Modus.
//
// Er gilt bis zum nächsten Neustart des Geräts; siehe [Kiosk]. Jede Feier
// bekommt eine eigene Kennung, und nur deren Fotos sind im Kiosk sichtbar.
// Das trennt nicht nur Feier und Mediathek, sondern auch zwei Feiern
// voneinander: Die Gäste am Samstag sehen nicht die Bilder vom Freitag.
type StartKiosk func(subject permission.Auditable, cmd StartKioskCmd) (Kiosk, error)

// NewStartKiosk erzeugt den [StartKiosk] Anwendungsfall.
func NewStartKiosk(mutex *sync.Mutex, settings SettingsStore, kiosk KioskStore, now func() time.Time) StartKiosk {
	return func(subject permission.Auditable, cmd StartKioskCmd) (Kiosk, error) {
		if err := subject.Audit(PermStartKiosk); err != nil {
			return Kiosk{}, err
		}

		current, err := kiosk.Load()
		if err != nil {
			return Kiosk{}, err
		}

		if current.Active() {
			return current, errors.New("der Kiosk läuft bereits")
		}

		title := strings.TrimSpace(cmd.Title)
		if title == "" {
			title = "Fotobox"
		}

		at := now()
		k := Kiosk{Event: photo.EventID(at.UTC().Format("20060102-150405")), Title: title, Since: at}

		// Zuerst die Feier eintragen, dann umschalten: Ein Kiosk, dessen
		// Feier nirgends steht, hinterließe Fotos ohne Namen.
		saved, err := modify(mutex, settings, func(s *Settings) {
			s.EventTitle = title
			s.Events = append(s.Events, Event{ID: k.Event, Title: title, StartedAt: at})
		})
		if err != nil {
			return Kiosk{}, err
		}

		k.Accent, k.Layouts, k.MaxCopies = saved.Accent, saved.KioskLayouts, saved.MaxCopies

		if err := kiosk.Save(k); err != nil {
			return Kiosk{}, err
		}

		return k, nil
	}
}
