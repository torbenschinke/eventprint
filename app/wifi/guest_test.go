package wifi_test

import (
	"errors"

	"go.wdy.de/nago/application/permission"
)

// guest ist ein Subject ohne jede Berechtigung.
type guest struct{}

func (guest) Audit(permission.ID) error { return errors.New("Zugriff verweigert") }

func (guest) HasPermission(permission.ID) bool { return false }
