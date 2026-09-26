package device

import (
	"fmt"

	"go.wdy.de/nago/application/permission"

	"github.com/torbenschinke/eventprint/app/photo"
)

// Role ist die Rolle dessen, der gerade vor dem Gerät steht.
type Role int

const (
	// RoleOwner bedient das Gerät im Heimbetrieb und darf alles.
	RoleOwner Role = iota

	// RoleGuest bedient das Gerät im Kiosk: Fotos der Feier ansehen und
	// drucken, sonst nichts.
	RoleGuest

	// RoleOperator betreut den Kiosk nach Eingabe der PIN.
	RoleOperator
)

func (r Role) String() string {
	switch r {
	case RoleOwner:
		return "Besitzer"
	case RoleGuest:
		return "Gast"
	case RoleOperator:
		return "Betreuung"
	default:
		return fmt.Sprintf("Rolle %d", int(r))
	}
}

// Grants legt fest, welche Berechtigungen eine Rolle hat.
//
// Die Liste stellt die Verdrahtung zusammen, weil nur sie alle Kontexte kennt.
// Besitzer und Betreuung haben dieselben Rechte; der Unterschied liegt darin,
// wann sie gelten.
type Grants struct {
	Owner []permission.ID
	Guest []permission.ID
}

// Actor ist das Subjekt eines Aufrufs.
type Actor struct {
	role    Role
	allowed map[permission.ID]struct{}
	event   photo.EventID
}

// WithEvent bindet das Subjekt an eine Feier. Für Gäste ist das die Schranke,
// hinter der sie Fotos sehen: die der laufenden Feier und keine anderen.
func (a Actor) WithEvent(e photo.EventID) Actor {
	a.event = e
	return a
}

// EventScope liefert die Feier, an die das Subjekt gebunden ist.
func (a Actor) EventScope() photo.EventID { return a.event }

// NewActor erzeugt das Subjekt einer Rolle.
func NewActor(role Role, grants Grants) Actor {
	ids := grants.Owner
	if role == RoleGuest {
		ids = grants.Guest
	}

	allowed := make(map[permission.ID]struct{}, len(ids))
	for _, id := range ids {
		allowed[id] = struct{}{}
	}

	return Actor{role: role, allowed: allowed}
}

// Role liefert die Rolle des Subjekts.
func (a Actor) Role() Role { return a.role }

// Audit prüft eine Berechtigung.
func (a Actor) Audit(id permission.ID) error {
	if a.HasPermission(id) {
		return nil
	}

	return PermissionDeniedError{Role: a.role, Permission: id}
}

// HasPermission meldet, ob die Rolle die Berechtigung hat.
func (a Actor) HasPermission(id permission.ID) bool {
	_, ok := a.allowed[id]
	return ok
}

// PermissionDeniedError meldet einen verweigerten Zugriff.
type PermissionDeniedError struct {
	Role       Role
	Permission permission.ID
}

func (e PermissionDeniedError) Error() string {
	return fmt.Sprintf("als %s nicht erlaubt (%s)", e.Role, e.Permission)
}
