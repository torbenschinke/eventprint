package photoupld

import (
	"context"
	"time"

	"go.wdy.de/nago/application"
	cfghapi "go.wdy.de/nago/application/hapi/cfg"
	"go.wdy.de/nago/application/role"
	"go.wdy.de/nago/application/settings"
	"go.wdy.de/nago/application/user"
	"go.wdy.de/nago/pkg/blob"
	"go.wdy.de/nago/presentation/core"

	uploadui "github.com/torbenschinke/eventprint/photoupld/ui"
	"github.com/torbenschinke/eventprint/upld"
)

const RelayRole role.ID = "de.torbenschinke.photoupld.relay"

type Management struct {
	UploadPage core.NavigationPath
	Registry   *upld.Registry
}

func Enable(cfg *application.Configurator) (Management, error) {
	images, err := cfg.ImageManagement()
	if err != nil {
		return Management{}, err
	}
	stores, err := cfg.Stores()
	if err != nil {
		return Management{}, err
	}
	setStore, err := stores.Open("nago.img.set", blob.OpenStoreOptions{Type: blob.EntityStore})
	if err != nil {
		return Management{}, err
	}
	imageStore, err := stores.Open("nago.img.blob", blob.OpenStoreOptions{Type: blob.FileStore})
	if err != nil {
		return Management{}, err
	}
	purger := upld.NewPurger(images.UseCases.LoadSrcSet, setStore, imageStore)
	if err := purger.DeleteOrphans(); err != nil {
		return Management{}, err
	}
	registry := upld.NewRegistry(purger.Delete)

	settingsMgmt, err := cfg.SettingsManagement()
	if err != nil {
		return Management{}, err
	}
	loadSettings := func() Settings { return settings.ReadGlobal[Settings](settingsMgmt.UseCases.LoadGlobal) }
	uploadURL := func(id upld.UploadID) string {
		return loadSettings().UploadURL(string(id), func() string { return cfg.ContextPathURI("", nil) })
	}

	roles, err := cfg.RoleManagement()
	if err != nil {
		return Management{}, err
	}
	if _, err := roles.UseCases.Upsert(user.SU(), role.Role{ID: RelayRole, Name: "Fotobox-Relay", Description: "Darf Uploads für genau eine Fotobox abrufen."}); err != nil {
		return Management{}, err
	}
	if err := roles.UseCases.UpdatePermissions(user.SU(), RelayRole, upld.RelayPermissions()); err != nil {
		return Management{}, err
	}

	tokens, err := cfg.TokenManagement()
	if err != nil {
		return Management{}, err
	}
	apiMgmt, err := cfghapi.Enable(cfg)
	if err != nil {
		return Management{}, err
	}
	ConfigureAPI(apiMgmt.API, tokens, images.UseCases, registry, uploadURL)

	const uploadPage core.NavigationPath = "upload"
	cfg.RootView(uploadPage, func(wnd core.Window) core.View {
		return uploadui.PageUpload(wnd, uploadui.Options{Registry: registry, CreateSrcSet: images.UseCases.CreateSrcSet})
	})
	go reap(cfg.Context(), registry)

	management := Management{UploadPage: uploadPage, Registry: registry}
	cfg.AddContextValue(core.ContextValue("eventprint.photoupld", management))
	return management, nil
}

// sessionIdleTimeout ist die Zeit ohne jeden Zugriff, nach der eine
// Upload-Sitzung verworfen wird.
//
// Gemessen wird der letzte Zugriff, nicht das Alter. Eine laufende Fotobox
// fragt ihre Sitzung regelmäßig ab und hält den QR-Code damit den ganzen
// Abend über gültig; verstummt sie, räumt der Verfall hinter ihr auf.
const sessionIdleTimeout = 30 * time.Minute

func reap(ctx context.Context, registry *upld.Registry) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			registry.PurgeOlderThan(time.Now().Add(-sessionIdleTimeout))
		}
	}
}
