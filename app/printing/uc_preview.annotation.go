package printing

import (
	"github.com/worldiety/speclink/spec"

	"github.com/torbenschinke/eventprint/requirements/fun/druck"
)

var _ = spec.For[Preview](
	spec.Satisfies(druck.RDruckVorschau),
	spec.Help(`Rendert ein Foto mit dem gewählten Layout, ohne zu drucken.`),
)
