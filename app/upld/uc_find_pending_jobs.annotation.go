package upld

import (
	"github.com/worldiety/speclink/spec"

	"github.com/torbenschinke/eventprint/requirements/fun/upload"
)

var _ = spec.For[FindPendingJobs](
	spec.Satisfies(upload.RUploadAbholung),
	spec.Help(`Liefert die Aufträge, die für die anfragende Fotobox bereitliegen.`),
)
