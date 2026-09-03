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
