package photoupld

import "github.com/worldiety/speclink/spec"

// POST /api/v1/oauth/adobe meldete eine Lightroom-Anmeldung an, die der
// Dienst an die Box weiterreichen sollte. Aufrufer war allein die Box selbst;
// die Lightroom-Anbindung ist samt Aufrufer entfernt, weil Adobe für eine Box
// ohne Browser keinen tragfähigen Anmeldeweg anbietet.
var _ = spec.ForPackage(
	spec.Waive("K20-ENDPOINT-REMOVED", "Die Lightroom-Anmeldung ist entfernt; einziger Aufrufer war die Box, deren Lightroom-Anbindung im selben Schritt entfiel."),
)
