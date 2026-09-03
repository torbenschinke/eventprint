package cfgphotobox

import (
	"sync"
	"time"
)

// TapsToConfigure ist die Zahl der Berührungen auf dem QR-Code, die den Zugang
// zur Einrichtung öffnet.
const TapsToConfigure = 5

// TapWindow ist der zulässige Abstand zwischen zwei Berührungen.
//
// Ohne dieses Fenster wäre die Hürde nur scheinbar eine: Über einen Abend
// tippen Gäste den QR-Code oft an, um zu sehen, was passiert. Diese
// Berührungen summierten sich, und irgendwann stünde die PIN-Eingabe offen,
// ohne dass jemand es beabsichtigt hätte. Mit dem Fenster zählt nur, wer
// zügig hintereinander tippt – also jemand, der die Griffkombination kennt.
const TapWindow = 2 * time.Second

// TapGate zählt Berührungen und öffnet bei genügend vielen in Folge.
//
// Der Zähler gehört zur Sitzung und nicht zur Anwendung: Zwei Gäste an zwei
// Geräten dürfen sich nicht gegenseitig zum Erfolg tippen.
type TapGate struct {
	mu     sync.Mutex
	needed int
	window time.Duration
	count  int
	last   time.Time

	now func() time.Time
}

// NewTapGate erstellt das Tor mit den betrieblichen Vorgaben.
func NewTapGate() *TapGate {
	return &TapGate{needed: TapsToConfigure, window: TapWindow, now: time.Now}
}

// Tap zählt eine Berührung und meldet, ob das Tor damit aufgeht.
//
// Bei Erfolg beginnt die Zählung von vorn, sonst genügte nach dem ersten Mal
// jede weitere Berührung.
func (g *TapGate) Tap() bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := g.now()

	// Eine zu späte Berührung ist keine Fortsetzung, sondern ein Anfang.
	if g.count > 0 && now.Sub(g.last) > g.window {
		g.count = 0
	}

	g.count++
	g.last = now

	if g.count >= g.needed {
		g.count = 0
		return true
	}

	return false
}

// Reset verwirft eine begonnene Folge.
func (g *TapGate) Reset() {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.count = 0
}

// tapGates führt je Sitzung ein eigenes Tor.
//
// Ein gemeinsames Tor für alle wäre falsch: Zwei Gäste an zwei Geräten dürfen
// sich nicht gegenseitig zum Erfolg tippen.
type tapGates struct {
	mu    sync.Mutex
	gates map[string]*TapGate
}

func newTapGates() *tapGates {
	return &tapGates{gates: map[string]*TapGate{}}
}

func (g *tapGates) Tap(sessionID string) bool {
	g.mu.Lock()
	gate, ok := g.gates[sessionID]
	if !ok {
		gate = NewTapGate()
		g.gates[sessionID] = gate
	}
	g.mu.Unlock()

	return gate.Tap()
}
