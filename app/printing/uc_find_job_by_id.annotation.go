package printing

import (
	"github.com/worldiety/speclink/spec"

	"github.com/torbenschinke/eventprint/requirements/fun/druck"
)

var _ = spec.For[FindJobByID](
	spec.Satisfies(druck.RDruckStatus),
	spec.Help(`Liefert einen einzelnen Druckauftrag anhand seiner Kennung.`),
)
