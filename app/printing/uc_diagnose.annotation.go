package printing

import (
	"github.com/worldiety/speclink/spec"

	"github.com/torbenschinke/eventprint/requirements/fun/druck"
)

var _ = spec.For[Diagnose](
	spec.Satisfies(druck.RDruckDiagnose),
	spec.Help(`Beschreibt den Zustand des Druckers für die Betreuung.`),
)
