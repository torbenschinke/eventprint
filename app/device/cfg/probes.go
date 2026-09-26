package cfgdevice

import (
	"context"
	"fmt"

	"go.wdy.de/nago/application/permission"

	"github.com/torbenschinke/eventprint/app/camera"
	"github.com/torbenschinke/eventprint/app/device"
	"github.com/torbenschinke/eventprint/app/relay"
)

// probes sind die Prüfungen vor dem Kiosk-Start.
func probes(d *Device, poller *relay.Poller, load func() device.Settings) []device.Probe {
	return []device.Probe{
		func(ctx context.Context, s device.Settings) device.Check {
			if s.Printer.TestMode() {
				return device.Check{Title: "Kein Drucker eingerichtet", Detail: "Aufträge laufen durch, gedruckt wird nichts", Level: device.LevelError}
			}

			st, err := d.Printing.Diagnose(permission.SU())
			if err != nil {
				return device.Check{Title: "Drucker nicht abfragbar", Detail: err.Error(), Level: device.LevelError}
			}

			if !st.OK() {
				return device.Check{Title: "Drucker nicht bereit", Detail: st.Problem(), Level: device.LevelError}
			}

			return device.Check{Title: "Drucker bereit", Detail: st.Queue, Level: device.LevelOK}
		},

		func(ctx context.Context, s device.Settings) device.Check {
			switch {
			case s.PaperLeft <= 0:
				return device.Check{Title: "Papier leer", Detail: "Bitte ein neues Set einlegen und in den Einstellungen melden", Level: device.LevelError}
			case s.PaperLeft < 10:
				return device.Check{Title: fmt.Sprintf("Nur noch %d Blatt", s.PaperLeft), Detail: "Ein Ersatzset bereitlegen", Level: device.LevelWarning}
			default:
				return device.Check{Title: fmt.Sprintf("%d Blatt Papier", s.PaperLeft), Detail: "Reicht für etwa so viele Ausdrucke", Level: device.LevelOK}
			}
		},

		func(ctx context.Context, s device.Settings) device.Check {
			switch poller.State() {
			case relay.StateReady:
				return device.Check{Title: "Handy-Upload erreichbar", Detail: "Der QR-Code führt über den Upload-Dienst", Level: device.LevelOK}
			case relay.StateOff:
				return device.Check{Title: "Kein Handy-Upload", Detail: "Gäste können nur über die Kamera drucken", Level: device.LevelWarning}
			default:
				return device.Check{Title: "Upload-Dienst nicht erreichbar", Detail: "Ohne Internet zeigt der Kiosk keinen QR-Code", Level: device.LevelWarning}
			}
		},

		func(ctx context.Context, s device.Settings) device.Check {
			if d.Camera == nil {
				return device.Check{Title: "Kamera abgeschaltet", Detail: "Nur Handy-Uploads", Level: device.LevelOK}
			}

			st := d.Camera.Status()
			if st.State == camera.StateConnected {
				return device.Check{Title: "Kamera verbunden", Detail: st.Model, Level: device.LevelOK}
			}

			return device.Check{Title: "Keine Kamera angeschlossen", Detail: "Gäste können nur vom Handy drucken", Level: device.LevelWarning}
		},
	}
}
