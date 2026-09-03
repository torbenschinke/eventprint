package photo

import (
	"github.com/worldiety/speclink/spec"

	"github.com/torbenschinke/eventprint/requirements/fun/archiv"
)

var _ = spec.For[ExportArchive](
	spec.Satisfies(archiv.RArchivExport),
	spec.Help(`Schreibt alle Bilder des Archivs als ZIP in den übergebenen Writer.
Strömend und nicht in den Speicher: Das Archiv einer Feier wiegt mehrere
Gigabyte, die Fotobox hat vier davon insgesamt. JPEG wird ohne weitere
Kompression abgelegt, weil erneutes Pressen nichts spart und Rechenzeit kostet.`),
)
