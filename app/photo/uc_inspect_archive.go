package photo

import (
	"os"

	"go.wdy.de/nago/auth"
)

// InspectArchive meldet Bestand und Platzbedarf des Fotoarchivs.
type InspectArchive func(subject auth.Subject) (ArchiveUsage, error)

// NewInspectArchive erzeugt den [InspectArchive] Anwendungsfall.
func NewInspectArchive(dir string) InspectArchive {
	return func(subject auth.Subject) (ArchiveUsage, error) {
		if err := subject.Audit(PermInspectArchive); err != nil {
			return ArchiveUsage{}, err
		}

		return usageOf(os.DirFS(dir))
	}
}
