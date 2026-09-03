package printing

import (
	"github.com/worldiety/speclink/spec"

	"github.com/torbenschinke/eventprint/requirements/fun/druck"
)

var _ = spec.For[FindAllJobs](
	spec.Satisfies(druck.RDruckStatus),
	spec.Help(`Liefert alle Druckaufträge, beginnend mit dem neuesten.`),
)
