package uiphotobox

import (
	"strings"

	"go.wdy.de/nago/presentation/core"
	"go.wdy.de/nago/presentation/ui"
	"go.wdy.de/nago/presentation/ui/alert"
)

// PinAccess bündelt alles, was das Tastenfeld über die PIN wissen muss.
//
// Es sind Funktionen und kein Verweis auf die Verwaltung, weil diese Schicht
// die Fachschicht nicht kennen darf: Die Einrichtung baut die Seiten, also
// zeigt die Abhängigkeit in genau eine Richtung. Ein Verweis zurück wäre ein
// Zyklus und ließe sich nicht übersetzen.
type PinAccess struct {
	// Configured meldet, ob überhaupt eine PIN vergeben ist. Ist sie es nicht,
	// steht das Gerät fabrikneu da und das Tastenfeld vergibt sie.
	Configured func() bool

	// Verify prüft eine eingegebene PIN und meldet die Sitzung als Betreuer an.
	//
	// Das Fenster statt nur der Sitzungskennung, weil die Anmeldung im
	// laufenden Fenster sofort wirken muss: Ohne das hätte die Seite die neuen
	// Rechte erst nach einem Neuladen.
	Verify func(wnd core.Window, pin string) error

	// Configure legt eine neue PIN fest und meldet die Sitzung an.
	Configure func(wnd core.Window, pin string) error

	// Tap zählt eine Berührung des QR-Codes und meldet, ob das Tor aufgeht.
	Tap func(sessionID string) bool

	// Lock meldet die Sitzung wieder ab.
	Lock func(sessionID string)
}

// pinLength ist die erwartete Stellenzahl. Sie steht hier gespiegelt, damit die
// Oberfläche nicht die Fachschicht importieren muss; der Wert ist Teil der
// Bedienung und ändert sich nicht ohne Weiteres.
const pinLength = 6

// pinPad ist die PIN-Eingabe auf dem Touchscreen.
//
// Ein eigenes Tastenfeld statt eines Textfeldes: Auf der Fotobox gibt es keine
// Tastatur, und die Bildschirmtastatur des Systems wäre ein Ausbruchsweg aus
// dem Kioskbetrieb. Große Ziffernflächen sind zudem das Einzige, was sich im
// Halbdunkel einer Feier zuverlässig treffen lässt.
type pinPad struct {
	wnd    core.Window
	access PinAccess

	presented *core.State[bool]
	entered   *core.State[string]
	repeat    *core.State[string]
	message   *core.State[string]
}

func newPinPad(wnd core.Window, access PinAccess) pinPad {
	return pinPad{
		wnd:       wnd,
		access:    access,
		presented: core.StateOf[bool](wnd, "pin-pad-presented"),
		entered:   core.StateOf[string](wnd, "pin-pad-entered"),
		repeat:    core.StateOf[string](wnd, "pin-pad-repeat"),
		message:   core.StateOf[string](wnd, "pin-pad-message"),
	}
}

// Open zeigt das Tastenfeld und verwirft eine frühere Eingabe.
func (p pinPad) Open() {
	p.entered.Set("")
	p.repeat.Set("")
	p.message.Set("")
	p.presented.Set(true)
}

// setupMode meldet, ob gerade eine PIN vergeben statt geprüft wird.
func (p pinPad) setupMode() bool {
	return p.access.Configured == nil || !p.access.Configured()
}

// View liefert den Dialog. Er rendert nur, solange er geöffnet ist, und gehört
// unverändert in jede Seite, die den Zugang anbieten soll.
func (p pinPad) View() core.View {
	if !p.presented.Get() {
		return nil
	}

	title := "PIN eingeben"
	if p.setupMode() {
		title = "PIN festlegen"
	}

	// Breite ausdruecklich gesetzt statt einer der Groessenstufen.
	//
	// Der Touchscreen der Fotobox misst 1024x600. Der Dialog war zuvor auf
	// 320 Punkte eingeschnuert, das Tastenfeld passte nicht hinein und liess
	// sich nur durch Scrollen erreichen - bei einer PIN-Eingabe die denkbar
	// schlechteste Bedienung.
	return alert.Dialog(
		title,
		p.body(),
		p.presented,
		alert.Width(ui.L480),
		alert.Cancel(nil),
	)
}

// body ist der Inhalt: Erklärung, Punktreihe, Tastenfeld, Fehlermeldung.
func (p pinPad) body() core.View {
	return ui.VStack(
		ui.Text(p.explanation()).
			Font(ui.BodyMedium).
			TextAlignment(ui.TextAlignCenter),

		p.dots(),
		p.errorText(),
		p.keys(),
	).
		Gap(ui.L12).
		Alignment(ui.Center).
		Frame(ui.Frame{}.FullWidth())
}

func (p pinPad) explanation() string {
	if !p.setupMode() {
		return "Betreuer-PIN eingeben."
	}

	if p.entered.Get() == "" || len(p.entered.Get()) < pinLength {
		return "Noch keine PIN vergeben. Bitte sechs Ziffern wählen und merken."
	}

	return "Zur Sicherheit noch einmal dieselbe PIN eingeben."
}

// active ist das Feld, in das gerade getippt wird.
func (p pinPad) active() *core.State[string] {
	if p.setupMode() && len(p.entered.Get()) == pinLength {
		return p.repeat
	}

	return p.entered
}

// dots zeigt, wie viele Stellen bereits eingegeben sind.
//
// Die Ziffern selbst erscheinen nie: Auf einer Feier steht selten jemand
// allein vor dem Bildschirm.
func (p pinPad) dots() core.View {
	filled := len(p.active().Get())

	var cells []core.View
	for i := range pinLength {
		colour := ui.M6
		if i < filled {
			colour = ui.M8
		}

		cells = append(cells, ui.HStack().
			BackgroundColor(colour).
			Border(ui.Border{}.Radius(ui.L8)).
			Frame(ui.Frame{}.Size(ui.L16, ui.L16)))
	}

	return ui.HStack(cells...).Gap(ui.L8)
}

func (p pinPad) errorText() core.View {
	msg := p.message.Get()
	if msg == "" {
		return nil
	}

	return ui.Text(msg).
		Font(ui.BodySmall).
		Color(ui.SE0).
		TextAlignment(ui.TextAlignCenter)
}

// keys ist das 3x4-Feld.
func (p pinPad) keys() core.View {
	var cells []ui.TGridCell

	for _, digit := range []string{"1", "2", "3", "4", "5", "6", "7", "8", "9"} {
		cells = append(cells, ui.GridCell(p.digitKey(digit)))
	}

	cells = append(cells,
		ui.GridCell(p.actionKey("C", func() {
			p.active().Set("")
			p.message.Set("")
		})),
		ui.GridCell(p.digitKey("0")),
		ui.GridCell(p.actionKey("<", func() {
			cur := p.active().Get()
			if cur != "" {
				p.active().Set(cur[:len(cur)-1])
			}

			p.message.Set("")
		})),
	)

	return ui.Grid(cells...).Columns(3).Gap(ui.L8)
}

// digitKey ist eine Zifferntaste. Sie prüft selbst, wann die Eingabe
// vollständig ist – so gibt es keinen zusätzlichen Bestätigungsknopf, den man
// im Halbdunkel erst suchen müsste.
func (p pinPad) digitKey(digit string) core.View {
	return p.key(digit, func() {
		field := p.active()

		if len(field.Get()) >= pinLength {
			return
		}

		field.Set(field.Get() + digit)
		p.message.Set("")

		if len(field.Get()) == pinLength {
			p.complete()
		}
	})
}

func (p pinPad) actionKey(label string, action func()) core.View {
	return p.key(label, action)
}

// key ist die gemeinsame Fläche aller Tasten.
//
// 64 Punkte Kantenlänge.
//
// Ein Kompromiss gegen die Bildschirmhöhe: Vier Reihen zu 80 Punkten passen
// samt Titel und Erklärung nicht in die 600 Punkte des Touchscreens, und der
// Dialog begänne zu scrollen. 64 Punkte sind rund 17 Millimeter und damit
// immer noch deutlich über dem, was ein Finger sicher trifft.
func (p pinPad) key(label string, action func()) core.View {
	return ui.VStack(
		ui.Text(label).Font(ui.TitleLarge),
	).
		BackgroundColor(ui.M4).
		Action(action).
		Border(ui.Border{}.Radius(ui.L12)).
		Frame(ui.Frame{}.Size(ui.L64, ui.L64))
}

// complete wertet eine vollständige Eingabe aus.
func (p pinPad) complete() {
	if p.setupMode() {
		p.completeSetup()
		return
	}

	if err := p.access.Verify(p.wnd, p.entered.Get()); err != nil {
		p.entered.Set("")
		p.message.Set(pinErrorText(err))

		return
	}

	p.presented.Set(false)
	p.entered.Set("")
}

// completeSetup verlangt die PIN zweimal.
//
// Ein Vertipper beim Vergeben wäre sonst nicht zu bemerken und sperrte die
// Fotobox dauerhaft zu – die PIN steht nirgends sonst.
func (p pinPad) completeSetup() {
	if len(p.repeat.Get()) < pinLength {
		return
	}

	if p.entered.Get() != p.repeat.Get() {
		p.entered.Set("")
		p.repeat.Set("")
		p.message.Set("Die beiden Eingaben stimmen nicht überein. Bitte noch einmal.")

		return
	}

	if err := p.access.Configure(p.wnd, p.entered.Get()); err != nil {
		p.entered.Set("")
		p.repeat.Set("")
		p.message.Set(pinErrorText(err))

		return
	}

	p.presented.Set(false)
	p.entered.Set("")
	p.repeat.Set("")
}

// pinErrorText bereitet einen Fehler für den Bildschirm auf.
//
// Der Text nennt den Grund, aber niemals, wie viele Stellen stimmten – jede
// solche Auskunft macht das Raten billiger.
func pinErrorText(err error) string {
	if err == nil {
		return ""
	}

	msg := err.Error()

	return strings.ToUpper(msg[:1]) + msg[1:]
}
