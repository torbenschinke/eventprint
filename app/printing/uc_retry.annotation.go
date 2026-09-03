package printing

import (
	"github.com/worldiety/speclink/spec"

	"github.com/torbenschinke/eventprint/requirements/fun/druck"
)

var _ = spec.For[Retry](
	spec.Satisfies(druck.RDruckWiederholung),
	spec.Help(`Stellt einen gescheiterten Auftrag erneut in die Warteschlange.
Ein noch anhängiger Auftrag desselben Bildes wird dabei zurückgenommen.`),
)
