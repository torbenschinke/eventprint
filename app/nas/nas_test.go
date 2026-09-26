package nas_test

import (
	"context"
	"errors"
	"io/fs"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/worldiety/speclink/spec"
	"go.wdy.de/nago/application/permission"

	"github.com/torbenschinke/eventprint/app/nas"
	"github.com/torbenschinke/eventprint/requirements/fun/quellen"
)

// guest ist ein Subject ohne jede Berechtigung.
type guest struct{}

func (guest) Audit(permission.ID) error { return errors.New("Zugriff verweigert") }

func (guest) HasPermission(permission.ID) bool { return false }

var su = permission.SU()

// fakeNAS ist eine DiskStation im Speicher: ein Benutzer, einige Freigaben,
// die Freigabe "photo" mit Inhalt.
type fakeNAS struct {
	password string
	shares   map[string]fstest.MapFS
	seen     []nas.Config
}

func (f *fakeNAS) login(cfg nas.Config) error {
	f.seen = append(f.seen, cfg)
	if cfg.Host != "diskstation" {
		return nas.ErrUnreachable
	}

	if cfg.User != "anna" || cfg.Password != f.password {
		return nas.ErrLogin
	}

	return nil
}

func (f *fakeNAS) Shares(_ context.Context, cfg nas.Config) ([]string, error) {
	if err := f.login(cfg); err != nil {
		return nil, err
	}

	names := []string{"IPC$", "video", "Photo-Archiv", "home", "ADMIN$"}
	for n := range f.shares {
		names = append(names, n)
	}

	return names, nil
}

func (f *fakeNAS) Do(_ context.Context, cfg nas.Config, fn func(fs.FS) error) error {
	if err := f.login(cfg); err != nil {
		return err
	}

	fsys, ok := f.shares[cfg.Share]
	if !ok {
		return nas.ErrShare
	}

	return fn(fsys)
}

func newNAS() (*fakeNAS, *nas.Config) {
	f := &fakeNAS{password: "geheim", shares: map[string]fstest.MapFS{
		"photo": {
			"2024/Urlaub/IMG_0002.JPG":                                {Data: []byte("original-2")},
			"2024/Urlaub/IMG_0001.heic":                               {Data: []byte("original-1")},
			"2024/Urlaub/notizen.txt":                                 {Data: []byte("kein bild")},
			"2024/Urlaub/._IMG_0001.heic":                             {Data: []byte("macos")},
			"2024/Urlaub/@eaDir/IMG_0001.heic/SYNOPHOTO_THUMB_XL.jpg": {Data: []byte("xl-1")},
			"2024/Urlaub/@eaDir/IMG_0001.heic/SYNOPHOTO_THUMB_M.jpg":  {Data: []byte("m-1")},
			"2024/Urlaub/@eaDir/IMG_0002.JPG/SYNOPHOTO_THUMB_M.jpg":   {Data: []byte("m-2")},
			"2024/Urlaub/#recycle/IMG_0003.jpg":                       {Data: []byte("gelöscht")},
			"2024/Hochzeit/IMG_0100.jpg":                              {Data: []byte("h")},
			"2023/IMG_5000.png":                                       {Data: []byte("p")},
			"#recycle/alt.jpg":                                        {Data: []byte("gelöscht")},
			".DS_Store":                                               {Data: []byte("x")},
			"lose.jpg":                                                {Data: []byte("l")},
		},
		"secret": {"geheim.jpg": {Data: []byte("s")}},
	}}

	cfg := &nas.Config{Host: "diskstation", User: "anna", Password: "geheim", Share: "photo"}

	return f, cfg
}

// Die Freigabe wird am Gerät eingerichtet: anmelden, Freigaben sehen, eine
// wählen. Das Kennwort muss dafür nur einmal getippt werden.
func TestSetUpShare(t *testing.T) {
	f, saved := newNAS()
	shares := nas.NewShares(f, func() nas.Config { return *saved })
	ctx := context.Background()

	// Die Adresse aus dem Finder bringt die Freigabe gleich mit.
	if got := (nas.Config{Host: " smb://diskstation/photo/ "}).Normalized(); got.Host != "diskstation" || got.Share != "photo" {
		t.Fatalf("smb URL: %+v", got)
	}

	if got := (nas.Config{Host: `\\diskstation\photo`}).Normalized(); got.Host != "diskstation" || got.Share != "photo" {
		t.Fatalf("UNC path: %+v", got)
	}

	names, err := shares(su, ctx, nas.Config{Host: "diskstation", User: "anna", Password: "geheim"})
	if err != nil {
		t.Fatal(err)
	}

	// Verwaltungsfreigaben fallen weg, der Rest ist sortiert wie im Finder.
	if want := []string{"home", "photo", "Photo-Archiv", "secret", "video"}; !slices.Equal(names, want) {
		t.Fatalf("shares = %v, want %v", names, want)
	}

	// Ohne Kennwort gilt das gespeicherte, solange es derselbe Benutzer
	// auf demselben NAS ist.
	if _, err := shares(su, ctx, nas.Config{Host: "diskstation", User: "anna"}); err != nil {
		t.Fatalf("saved password must be used: %v", err)
	}

	// Für einen anderen Benutzer gilt es nicht.
	if _, err := shares(su, ctx, nas.Config{Host: "diskstation", User: "ben"}); !errors.Is(err, nas.ErrLogin) {
		t.Fatalf("other user must not get the saved password, got %v", err)
	}

	if last := f.seen[len(f.seen)-1]; last.Password != "" {
		t.Fatalf("saved password leaked to another user: %+v", last)
	}

	if _, err := shares(su, ctx, nas.Config{Host: "diskstation", User: "anna", Password: "falsch"}); !errors.Is(err, nas.ErrLogin) {
		t.Fatalf("wrong password: %v", err)
	}

	if _, err := shares(su, ctx, nas.Config{}); !errors.Is(err, nas.ErrNotConfigured) {
		t.Fatalf("empty config: %v", err)
	}

	if _, err := shares(guest{}, ctx, *saved); err == nil {
		t.Fatal("guest must be denied")
	}

	spec.Verified(t, quellen.RQuellenNas)
}

// Ordner und Bilder lassen sich durchsuchen; Verwaltungsordner, versteckte
// Dateien und alles, was kein Bild ist, bleiben unsichtbar.
func TestBrowseShare(t *testing.T) {
	f, cfg := newNAS()
	uc := nas.NewUseCases(f, func() nas.Config { return *cfg })
	ctx := context.Background()

	root, err := uc.Browse(su, ctx, "")
	if err != nil {
		t.Fatal(err)
	}

	if root.Dir != "." || !slices.Equal(folderNames(root), []string{"2023", "2024"}) || !slices.Equal(imageNames(root), []string{"lose.jpg"}) {
		t.Fatalf("root = %+v", root)
	}

	l, err := uc.Browse(su, ctx, "/2024/Urlaub/")
	if err != nil {
		t.Fatal(err)
	}

	if len(l.Folders) != 0 || !slices.Equal(imageNames(l), []string{"IMG_0001.heic", "IMG_0002.JPG"}) {
		t.Fatalf("Urlaub = %+v", l)
	}

	if l.Images[0].Path != "2024/Urlaub/IMG_0001.heic" || l.Images[0].Size != int64(len("original-1")) {
		t.Fatalf("image = %+v", l.Images[0])
	}

	// Nichts, was von der Oberfläche zurückkommt, führt aus der Freigabe
	// oder in ihre Verwaltungsordner.
	for _, bad := range []string{"../etc", "2024/../../x", "2024/Urlaub/@eaDir", "#recycle", ".ssh"} {
		if _, err := uc.Browse(su, ctx, bad); err == nil {
			t.Errorf("Browse(%q) must fail", bad)
		}
	}

	if _, err := uc.Browse(guest{}, ctx, ""); err == nil {
		t.Fatal("guest must be denied")
	}

	// Ohne eingerichtete Freigabe gibt es eine Meldung, die sagt, wo man
	// sie einrichtet.
	empty := nas.NewUseCases(f, func() nas.Config { return nas.Config{Host: "diskstation"} })
	if _, err := empty.Browse(su, ctx, ""); !errors.Is(err, nas.ErrNotConfigured) || !strings.Contains(err.Error(), "Einstellungen") {
		t.Fatalf("unconfigured: %v", err)
	}

	spec.Verified(t, quellen.RQuellenNas)
}

// Vorschaubilder kommen aus @eaDir, das Original erst, wenn es keine gibt.
// Übernommen wird immer das Original, und nur von der eingerichteten
// Freigabe.
func TestThumbnailsAndRead(t *testing.T) {
	f, cfg := newNAS()
	uc := nas.NewUseCases(f, func() nas.Config { return *cfg })
	ctx := context.Background()

	for p, want := range map[string]string{
		"2024/Urlaub/IMG_0001.heic":  "xl-1",
		"2024/Urlaub/IMG_0002.JPG":   "m-2",
		"2024/Hochzeit/IMG_0100.jpg": "h",
	} {
		got, err := uc.Thumbnail(su, ctx, p)
		if err != nil || string(got) != want {
			t.Errorf("Thumbnail(%q) = %q, %v; want %q", p, got, err, want)
		}
	}

	data, err := uc.Read(su, ctx, "2024/Urlaub/IMG_0001.heic")
	if err != nil || string(data) != "original-1" {
		t.Fatalf("Read = %q, %v", data, err)
	}

	for _, bad := range []string{"2024/Urlaub/notizen.txt", "../secret/geheim.jpg", "2024/Urlaub/#recycle/IMG_0003.jpg", "2024/Urlaub/@eaDir/IMG_0001.heic/SYNOPHOTO_THUMB_XL.jpg"} {
		if _, err := uc.Read(su, ctx, bad); err == nil {
			t.Errorf("Read(%q) must fail", bad)
		}
	}

	if _, err := uc.Read(su, ctx, "2024/fehlt.jpg"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing file: %v", err)
	}

	if _, err := uc.Thumbnail(guest{}, ctx, "lose.jpg"); err == nil {
		t.Fatal("guest must be denied thumbnails")
	}

	if _, err := uc.Read(guest{}, ctx, "lose.jpg"); err == nil {
		t.Fatal("guest must be denied reading")
	}

	spec.Verified(t, quellen.RQuellenNas)
}

func folderNames(l nas.Listing) []string {
	var out []string
	for _, f := range l.Folders {
		out = append(out, f.Name)
	}

	return out
}

func imageNames(l nas.Listing) []string {
	var out []string
	for _, i := range l.Images {
		out = append(out, i.Name)
	}

	return out
}
