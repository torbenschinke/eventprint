package device

import (
	"fmt"

	"go.wdy.de/nago/application/permission"

	"github.com/torbenschinke/eventprint/app/photo"
	"github.com/torbenschinke/eventprint/app/printing"
)

// IntakeCmd beschreibt ein Bild, das von außen ankommt – vom Handy über den
// Upload-Dienst oder von der Kamera.
type IntakeCmd struct {
	Name   string
	Source photo.Source
	Data   []byte

	// Template ist das Layout, das ein Gast auf dem Handy gewählt hat. Leer
	// bedeutet: keine Wahl, etwa weil der Upload aus dem Heimbetrieb stammt.
	Template printing.TemplateID
}

// IntakeResult sagt, was mit dem Bild geschehen ist.
type IntakeResult struct {
	Photo   photo.Photo
	Printed bool
}

// Intake nimmt ein Bild an und entscheidet nach Betriebsart, was damit
// geschieht.
//
// Im Heimbetrieb landet es im Eingang und wartet, bis jemand Format und
// Gestaltung wählt – gedruckt wird nichts von allein. Im Kiosk gehört es zur
// laufenden Feier und wird, je nach Einstellung, sofort gedruckt. Diese
// Unterscheidung steht an genau einer Stelle, damit Handy und Kamera sich nie
// unterschiedlich verhalten.
type Intake func(subject permission.Auditable, cmd IntakeCmd) (IntakeResult, error)

// NewIntake erzeugt den [Intake] Anwendungsfall.
//
// Import und Druck laufen als System: Wer das Bild geschickt hat, steht nicht
// vor dem Gerät, und die Entscheidung ist hier bereits gefallen.
func NewIntake(settings SettingsStore, kiosk KioskStore, importPhoto photo.Import, print printing.PrintSimple) Intake {
	return func(subject permission.Auditable, cmd IntakeCmd) (IntakeResult, error) {
		if err := subject.Audit(PermIntake); err != nil {
			return IntakeResult{}, err
		}

		k, err := kiosk.Load()
		if err != nil {
			return IntakeResult{}, err
		}

		if !k.Active() {
			p, err := importPhoto(permission.SU(), photo.ImportCmd{Name: cmd.Name, Source: cmd.Source, Unseen: true, Data: cmd.Data})
			return IntakeResult{Photo: p}, err
		}

		p, err := importPhoto(permission.SU(), photo.ImportCmd{Name: cmd.Name, Source: cmd.Source, Event: k.Event, Data: cmd.Data})
		if err != nil {
			return IntakeResult{}, err
		}

		s, err := settings.Load()
		if err != nil {
			return IntakeResult{Photo: p}, err
		}

		s = s.Normalized()

		auto := (cmd.Source == photo.SourceCamera && s.PrintCamera) || (cmd.Source != photo.SourceCamera && s.PrintUploads)
		if !auto {
			return IntakeResult{Photo: p}, nil
		}

		tpl := cmd.Template
		if tpl == "" || !allowed(s.KioskLayouts, tpl) {
			tpl = s.KioskDefault
		}

		if _, err := print(permission.SU(), printing.SimpleCmd{Photo: p.ID, Template: tpl, Copies: 1}); err != nil {
			return IntakeResult{Photo: p}, fmt.Errorf("cannot print incoming photo: %w", err)
		}

		return IntakeResult{Photo: p, Printed: true}, nil
	}
}

func allowed(list []printing.TemplateID, tpl printing.TemplateID) bool {
	for _, t := range list {
		if t == tpl {
			return true
		}
	}

	return false
}
