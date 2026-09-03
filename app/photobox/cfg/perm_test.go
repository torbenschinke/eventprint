package cfgphotobox

import (
	"strings"
	"testing"

	"go.wdy.de/nago/application/permission"
)

// permissionPrefix ist der Namensraum der Berechtigungen dieser Anwendung.
// permission.All() liefert auch die von nago selbst; die verteilt nago.
const permissionPrefix = "de.torbenschinke.eventprint."

// TestEveryPermissionReachesARole schließt eine Lücke, die sonst niemand sieht.
//
// Eine Berechtigung, die niemandem zugeteilt ist, bricht nichts beim
// Übersetzen und nichts beim Start. Sie führt dazu, dass die betroffene Seite
// im Betrieb schlicht leer bleibt oder eine Fehlermeldung zeigt – und zwar
// erst dann, wenn ein Gast davorsteht.
//
// Genau das ist beim Umbau passiert: Vier neue Berechtigungen entstanden mit
// ihren Anwendungsfällen, und keine davon landete in einer Rolle. Der
// Startbildschirm blieb daraufhin leer. Aufgefallen ist es erst im
// Browsertest, obwohl die Ursache eine reine Go-Tatsache ist.
func TestEveryPermissionReachesARole(t *testing.T) {
	granted := map[permission.ID]struct{}{}
	for _, id := range operatorPermissions() {
		granted[id] = struct{}{}
	}

	var declared int

	for p := range permission.All() {
		id := p.Identity()
		if !strings.HasPrefix(string(id), permissionPrefix) {
			continue
		}

		declared++

		if _, ok := granted[id]; !ok {
			t.Errorf("die Berechtigung %s ist deklariert, aber keiner Rolle zugeteilt.\n"+
				"Trag sie in guestPermissions() ein, oder in operatorPermissions(), wenn sie der Bedienung vorbehalten bleiben soll.", id)
		}
	}

	// Sonst liefe der Test grün, weil nichts geladen wurde.
	if declared == 0 {
		t.Fatal("es wurde keine einzige Berechtigung dieser Anwendung gefunden")
	}
}

// TestGuestIsNotAnOperator hält die Grenze zwischen beiden Rollen fest.
//
// Ohne diese Prüfung könnte eine Berechtigung versehentlich in
// guestPermissions() landen, und jeder anonyme Besucher dürfte Fotos löschen.
func TestGuestIsNotAnOperator(t *testing.T) {
	guest := map[permission.ID]struct{}{}
	for _, id := range guestPermissions() {
		guest[id] = struct{}{}
	}

	// Was ausschließlich der Bedienung gehört. Wächst die Liste, gehört der
	// neue Eintrag bewusst hierher oder bewusst nicht.
	operatorOnly := []permission.ID{
		"de.torbenschinke.eventprint.photo.delete",
		"de.torbenschinke.eventprint.printing.retry",
	}

	for _, id := range operatorOnly {
		if _, ok := guest[id]; ok {
			t.Errorf("%s ist für die Bedienung gedacht, steht aber in den Gastrechten", id)
		}
	}
}
