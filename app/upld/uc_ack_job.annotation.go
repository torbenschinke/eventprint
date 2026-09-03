package upld

import (
	"github.com/worldiety/speclink/spec"

	"github.com/torbenschinke/eventprint/requirements/fun/upload"
)

var _ = spec.For[AckJob](
	spec.Satisfies(upload.RUploadBestaetigung),
	spec.Help(`Bestätigt einen übernommenen Auftrag und entfernt ihn samt Bild.`),
)
