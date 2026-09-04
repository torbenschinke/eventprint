package camera

import (
	"sync"
	"time"
)

// State beschreibt, woran die Kameraanbindung gerade ist.
type State string

const (
	// StateSearching bedeutet: es ist keine Kamera am USB zu finden.
	StateSearching State = "searching"

	// StateConnected bedeutet: das Tethering läuft, ein Auslösen kommt an.
	StateConnected State = "connected"

	// StateError bedeutet: eine Kamera ist da, aber gphoto2 kommt nicht an sie
	// heran. Das ist der Zustand, in dem Aufnahmen verloren gehen, deshalb
	// muss er sichtbar sein.
	StateError State = "error"
)

// Status ist die Auskunft der Kamera an die Oberfläche.
//
// Vorher gab es sie nicht. Fiel das Tethering aus, stand das als Warnung im
// Log und sonst nirgends: Der Gast stand vor einer Box, die stumm blieb, und
// löste weiter aus, während die Bilder nur noch auf der Speicherkarte landeten.
type Status struct {
	State State

	// Model ist die von gphoto2 gemeldete Bezeichnung, etwa "Canon EOS 80D".
	Model string

	// Port ist der USB-Anschluss, etwa "usb:001,004".
	Port string

	// Detail erklärt einen Fehler in einem Satz, für die Betreuung gedacht.
	Detail string

	// Captures zählt die seit dem Start übernommenen Aufnahmen.
	Captures int

	// LastCapture ist der Zeitpunkt der letzten übernommenen Aufnahme.
	LastCapture time.Time

	// Pending ist die Zahl der Bilder, die noch importiert oder gedruckt
	// werden.
	Pending int
}

// status hält den Zustand für die Oberfläche fest.
//
// Auf ihn schreiben der Supervisor und der Worker, gelesen wird er aus dem
// Renderthread der Oberfläche, deshalb der Mutex.
type status struct {
	mu    sync.Mutex
	value Status
}

func (s *status) get() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.value
}

func (s *status) update(fn func(*Status)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(&s.value)
}

func (s *status) searching() {
	s.update(func(v *Status) {
		v.State = StateSearching
		v.Model, v.Port, v.Detail = "", "", ""
	})
}

func (s *status) connected(model, port string) {
	s.update(func(v *Status) {
		v.State = StateConnected
		v.Model, v.Port, v.Detail = model, port, ""
	})
}

func (s *status) failed(model, port, detail string) {
	s.update(func(v *Status) {
		v.State = StateError
		v.Model, v.Port, v.Detail = model, port, detail
	})
}

func (s *status) captured() {
	s.update(func(v *Status) {
		v.Captures++
		v.LastCapture = time.Now()
	})
}

func (s *status) pending(n int) {
	s.update(func(v *Status) { v.Pending = n })
}
