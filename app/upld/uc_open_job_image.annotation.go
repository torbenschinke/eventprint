package upld

import (
	"github.com/worldiety/speclink/spec"

	"github.com/torbenschinke/eventprint/requirements/fun/upload"
)

var _ = spec.For[OpenJobImage](
	spec.Satisfies(upload.RUploadBild),
	spec.Help(`Öffnet das Originalbild eines wartenden Auftrags.`),
)
