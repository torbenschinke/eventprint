// Package photo verwaltet die Fotos des Geräts: Import aus allen Quellen,
// Ablage der Originale, Auswahl für den Druck und Weitergabe.
//
// Woher ein Bild kommt – Kamera, Handy, Lightroom, USB-Stick –, spielt nach
// dem Import keine Rolle mehr. Es liegt als unverändertes Original auf der
// Speicherkarte, und alles Weitere (Vorschau, Druck, Export) liest genau diese
// Datei.
package photo

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"go.wdy.de/nago/application/permission"

	"go.wdy.de/nago/pkg/data"
)

// ID identifiziert ein Foto.
//
// Die ID ist zeitlich sortierbar aufgebaut (Unix-Millis, links mit Nullen
// aufgefüllt, gefolgt von Zufall). Die lexikographisch sortierte Iteration des
// Repositories liefert dadurch die chronologische Reihenfolge ohne Index.
type ID string

// NewID erzeugt eine neue, zeitlich sortierbare ID für den Zeitpunkt t.
//
// Die Millisekunde ist streng steigend: Ein Stapel vom USB-Stick kommt
// schneller herein, als die Uhr tickt, und der Zufallsteil allein ordnete
// Bilder derselben Millisekunde beliebig.
func NewID(t time.Time) ID {
	var buf [8]byte
	_, _ = rand.Read(buf[:])

	ms := t.UTC().UnixMilli()
	idClock.Lock()
	if ms <= idClock.last {
		ms = idClock.last + 1
	}
	idClock.last = ms
	idClock.Unlock()

	return ID(fmt.Sprintf("%013d-%s", ms, hex.EncodeToString(buf[:])))
}

var idClock struct {
	sync.Mutex
	last int64
}

// EventScoped wird von Subjekten erfüllt, die an eine Feier gebunden sind –
// den Gästen im Kiosk. Wer nicht die ganze Mediathek sehen darf, sieht genau
// die Fotos dieser Feier und keine anderen.
type EventScoped interface {
	EventScope() EventID
}

// visibleTo entscheidet, ob ein Subjekt ohne Mediatheksrecht ein Foto sehen
// darf: nur ein Foto der Feier, an die es gebunden ist. Private Fotos und die
// Fotos früherer Feiern bleiben verborgen, auch wenn jemand ihre Kennung
// kennt.
func visibleTo(subject interface{ HasPermission(id permission.ID) bool }, p Photo) bool {
	if subject.HasPermission(PermFindAll) {
		return true
	}

	scoped, ok := subject.(EventScoped)
	return ok && !p.Private() && p.Event == scoped.EventScope()
}

// EventID ordnet ein Foto einer Feier zu, also einem Lauf des Kiosk-Modus.
//
// Leer bedeutet: privat, im Heimbetrieb entstanden. Die Unterscheidung ist
// keine Ordnungshilfe, sondern eine Schranke. Im Kiosk sehen Gäste
// ausschließlich die Fotos der laufenden Feier – nie die private Mediathek.
type EventID string

// Source beschreibt, woher ein Foto stammt.
type Source string

const (
	// SourceCamera markiert Aufnahmen der angeschlossenen Kamera.
	SourceCamera Source = "camera"

	// SourceRelay markiert Uploads vom Handy über den öffentlichen
	// Upload-Dienst.
	SourceRelay Source = "relay"

	// SourceLightroom markiert Bilder aus Adobe Lightroom.
	SourceLightroom Source = "lightroom"

	// SourceUSB markiert Bilder von einem USB-Stick.
	SourceUSB Source = "usb"
)

// String liefert den Anzeigenamen.
func (s Source) String() string {
	switch s {
	case SourceCamera:
		return "Kamera"
	case SourceRelay:
		return "Handy"
	case SourceLightroom:
		return "Lightroom"
	case SourceUSB:
		return "USB-Stick"
	default:
		return string(s)
	}
}

// Photo ist das Aggregat eines einzelnen Bildes.
type Photo struct {
	ID ID `json:"id,omitempty"`

	// Name ist der ursprüngliche Dateiname, sofern bekannt. Er wird beim
	// Export auf den USB-Stick wiederverwendet, damit die Gäste ihre Bilder
	// wiedererkennen.
	Name string `json:"name,omitempty"`

	// File ist der Dateiname des Originals im Ablageverzeichnis.
	File string `json:"file,omitempty"`

	Source Source `json:"src,omitempty"`

	// Event ist die Feier, auf der das Foto entstanden ist; leer bei privaten
	// Fotos.
	Event EventID `json:"event,omitempty"`

	// Unseen markiert ein Foto, das noch im Eingang liegt: angekommen, aber
	// weder angesehen noch gedruckt.
	Unseen bool `json:"unseen,omitempty"`

	Favorite bool `json:"fav,omitempty"`

	// Prints zählt die erfolgreich gedruckten Blätter mit diesem Foto.
	Prints int `json:"prints,omitempty"`

	// Width und Height sind die Maße in der Lage, in der das Bild betrachtet
	// wird – also nach Auswertung der EXIF-Ausrichtung.
	Width  int `json:"w,omitempty"`
	Height int `json:"h,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
}

func (p Photo) Identity() ID { return p.ID }

func (p Photo) WithIdentity(id ID) Photo {
	p.ID = id
	return p
}

func (p Photo) String() string {
	if p.Name != "" {
		return p.Name
	}

	return string(p.ID)
}

// Landscape meldet, ob das Foto im Querformat vorliegt.
func (p Photo) Landscape() bool { return p.Width >= p.Height }

// Private meldet, ob das Foto zu keiner Feier gehört.
func (p Photo) Private() bool { return p.Event == "" }

// Repository speichert die Metadaten aller Fotos.
type Repository = data.Repository[Photo, ID]

// Scope wählt aus, welche Fotos eine Liste zeigt.
type Scope int

const (
	// ScopeAll sind alle Fotos, private wie die aller Feiern.
	ScopeAll Scope = iota

	// ScopeInbox sind private Fotos, die von selbst angekommen sind: vom
	// Handy über den Upload-Dienst oder von der Kamera.
	ScopeInbox

	// ScopeFavorites sind die markierten Fotos.
	ScopeFavorites

	// ScopePrinted sind Fotos, die schon einmal gedruckt wurden.
	ScopePrinted

	// ScopeEvent sind die Fotos einer bestimmten Feier.
	ScopeEvent
)

// Query beschreibt eine Liste von Fotos.
type Query struct {
	Scope Scope

	// Event ist bei [ScopeEvent] die gewünschte Feier.
	Event EventID

	// Limit begrenzt die Zahl der Treffer; 0 bedeutet unbegrenzt.
	Limit int
}

// matches entscheidet, ob ein Foto zur Abfrage gehört.
func (q Query) matches(p Photo) bool {
	switch q.Scope {
	case ScopeInbox:
		return p.Private() && (p.Source == SourceRelay || p.Source == SourceCamera)
	case ScopeFavorites:
		return p.Favorite
	case ScopePrinted:
		return p.Prints > 0
	case ScopeEvent:
		return p.Event == q.Event
	default:
		return true
	}
}
