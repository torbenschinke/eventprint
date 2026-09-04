package uiphotobox

import (
	"fmt"
	"time"

	"go.wdy.de/nago/presentation/core"
	"go.wdy.de/nago/presentation/ui"

	"github.com/torbenschinke/eventprint/app/photobox/cfg/camera"
)

// cameraStatusView zeigt unter dem QR-Code, wie es der Kamera geht.
//
// Der Startbildschirm verriet darüber vorher nichts. Riss das Tethering ab,
// stand das als Warnung im Log, und vor dem Gerät sah alles normal aus: Es
// wurde weiter ausgelöst, die Bilder blieben auf der Speicherkarte, und
// aufgefallen ist es erst, als am Ende des Abends welche fehlten.
//
// Deshalb steht hier auch im Normalfall eine Zeile. Ein Hinweis, der nur im
// Fehlerfall erscheint, wird beim Aufbau nicht geprüft - eine Zeile, die
// immer da ist, fällt sofort auf, wenn sie die Farbe wechselt.
func cameraStatusView(opts Options) core.View {
	if opts.CameraStatus == nil {
		return nil
	}

	status := opts.CameraStatus()
	color, headline, detail := cameraStatusText(status)

	lines := []core.View{
		ui.HStack(
			// Der Punkt trägt die Aussage auf Entfernung. Die Fotobox steht
			// im Raum, und niemand liest aus zwei Metern eine Kleinschrift.
			ui.VStack().
				BackgroundColor(color).
				Border(ui.Border{}.Circle()).
				Frame(ui.Frame{}.Size(ui.L12, ui.L12)),
			ui.Text(headline).Font(ui.BodySmall).Color(color),
		).Gap(ui.L8).Alignment(ui.Center),
	}

	if detail != "" {
		lines = append(lines, ui.Text(detail).Font(ui.BodySmall).
			TextAlignment(ui.TextAlignCenter))
	}

	return ui.VStack(lines...).
		Gap(ui.L4).
		Alignment(ui.Center).
		FullWidth().
		AccessibilityLabel("Zustand der Kamera")
}

// cameraStatusText übersetzt den Zustand in Farbe, Überschrift und Erklärung.
func cameraStatusText(status camera.Status) (ui.Color, string, string) {
	switch status.State {
	case camera.StateConnected:
		headline := "Kamera bereit"
		if status.Model != "" {
			headline = "Kamera bereit: " + status.Model
		}

		return ui.ColorSemanticGood, headline, cameraActivity(status)

	case camera.StateError:
		// Das ist der teure Zustand: Wer jetzt auslöst, verliert das Bild.
		// Also wird das auch so gesagt, statt von einem "Fehler" zu sprechen.
		return ui.ColorSemanticError, "Kamera nicht erreichbar",
			cameraProblem(status)

	default:
		return ui.ColorSemanticWarn, "Kamera wird gesucht",
			"Kamera einschalten und per USB anschließen."
	}
}

func cameraProblem(status camera.Status) string {
	detail := status.Detail
	if detail == "" {
		detail = "gphoto2 bekommt keine Verbindung."
	}

	return detail + " Aufnahmen aus dieser Zeit bleiben auf der Speicherkarte."
}

// cameraActivity berichtet, was die Kamera bisher geliefert hat.
func cameraActivity(status camera.Status) string {
	if status.Pending > 0 {
		return fmt.Sprintf("%s wird verarbeitet.", plural(status.Pending, "Aufnahme", "Aufnahmen"))
	}

	if status.Captures == 0 {
		return "Noch keine Aufnahme. Einfach auslösen."
	}

	return fmt.Sprintf("%s übernommen, zuletzt %s.",
		plural(status.Captures, "Aufnahme", "Aufnahmen"), ago(status.LastCapture))
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}

	return fmt.Sprintf("%d %s", n, many)
}

// ago beschreibt einen Zeitpunkt so, wie man ihn im Vorbeigehen liest.
func ago(at time.Time) string {
	if at.IsZero() {
		return "gerade eben"
	}

	switch d := time.Since(at); {
	case d < time.Minute:
		return "gerade eben"
	case d < time.Hour:
		return fmt.Sprintf("vor %d Min.", int(d.Minutes()))
	default:
		return at.Format("15:04 Uhr")
	}
}
