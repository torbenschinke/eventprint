package upld

import (
	"github.com/worldiety/speclink/spec"

	"github.com/torbenschinke/eventprint/requirements/fun/upload"
)

var _ = spec.For[OpenSession](
	spec.Satisfies(upload.RUploadSitzung),
	spec.Help(`Legt eine neue, kurzlebige Upload-Identität für die anfragende
Fotobox an. Eine bestehende Sitzung derselben Fotobox wird dabei verworfen.`),
)

var _ = spec.For[FindPendingJobs](
	spec.Satisfies(upload.RUploadAbholung),
	spec.Help(`Liefert die Aufträge, die für die anfragende Fotobox bereitliegen.`),
)

var _ = spec.For[OpenJobImage](
	spec.Satisfies(upload.RUploadBild),
	spec.Help(`Öffnet das Originalbild eines wartenden Auftrags.`),
)

var _ = spec.For[AckJob](
	spec.Satisfies(upload.RUploadBestaetigung),
	spec.Help(`Bestätigt einen übernommenen Auftrag und entfernt ihn samt Bild.`),
)
