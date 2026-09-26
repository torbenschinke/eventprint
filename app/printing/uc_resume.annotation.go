package printing

import (
	"github.com/worldiety/speclink/spec"

	"github.com/torbenschinke/eventprint/requirements/fun/druck"
)

var _ = spec.For[Resume](
	spec.Satisfies(druck.RDruckFreigabe),
	spec.Help(`Gibt einen angehaltenen Drucker wieder frei, etwa nach einem Papierwechsel.`),
)
