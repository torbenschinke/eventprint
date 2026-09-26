package usb_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.wdy.de/nago/application/permission"
	"go.wdy.de/nago/application/user"

	"github.com/torbenschinke/eventprint/app/usb"
)

// guest ist ein Subject ohne jede Berechtigung.
type guest struct{}

func (guest) Audit(permission.ID) error { return errors.New("Zugriff verweigert") }

func (guest) HasPermission(permission.ID) bool { return false }

// fakeStick spielt lsblk und udisksctl für einen Pi mit Speicherkarte und
// einem Stick /dev/sda1. Er merkt sich, ob der Stick eingehängt ist, damit
// das Einhängen bei Bedarf wirklich geprüft wird und nicht nur behauptet.
//
// Als Einhängepunkt dient ein temporäres Verzeichnis; so laufen die Tests
// auf jedem Rechner, ohne einen echten Stick anzufassen.
type fakeStick struct {
	mount   string
	mounted bool

	// mountMsg ist die Antwort von udisksctl mount; %s wird durch den
	// Einhängepunkt ersetzt.
	mountMsg   string
	unmountOut string
	unmountErr error
	powerErr   error

	calls []string
}

func newStick(t *testing.T) *fakeStick {
	t.Helper()

	return &fakeStick{mount: t.TempDir(), mountMsg: "Mounted /dev/sda1 at %s.\n"}
}

func (s *fakeStick) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	cmd := strings.Join(append([]string{name}, args...), " ")
	s.calls = append(s.calls, cmd)

	switch {
	case name == "lsblk":
		return s.lsblk(), nil
	case strings.HasPrefix(cmd, "udisksctl mount"):
		s.mounted = true
		return []byte(fmt.Sprintf(s.mountMsg, s.mount)), nil
	case strings.HasPrefix(cmd, "udisksctl unmount"):
		if s.unmountErr != nil {
			return []byte(s.unmountOut), s.unmountErr
		}

		s.mounted = false

		return []byte("Unmounted /dev/sda1.\n"), nil
	case strings.HasPrefix(cmd, "udisksctl power-off"):
		return nil, s.powerErr
	}

	return nil, fmt.Errorf("unerwarteter Aufruf %q", cmd)
}

func (s *fakeStick) lsblk() []byte {
	mp := "null"
	if s.mounted {
		b, _ := json.Marshal(s.mount)
		mp = string(b)
	}

	return []byte(`{"blockdevices":[
	  {"name":"sda","path":"/dev/sda","rm":true,"hotplug":true,"tran":"usb","type":"disk","fstype":null,"label":null,"size":15931539456,"mountpoint":null,"model":"Cruzer Blade","vendor":"SanDisk",
	   "children":[{"name":"sda1","path":"/dev/sda1","rm":true,"hotplug":true,"tran":null,"type":"part","fstype":"vfat","label":"PARTY","size":15929442304,"mountpoint":` + mp + `,"model":null,"vendor":null}]},
	  {"name":"mmcblk0","path":"/dev/mmcblk0","rm":false,"hotplug":false,"tran":null,"type":"disk","fstype":null,"label":null,"size":31914983424,"mountpoint":null,"model":null,"vendor":null,
	   "children":[{"name":"mmcblk0p2","path":"/dev/mmcblk0p2","rm":false,"hotplug":false,"tran":null,"type":"part","fstype":"ext4","label":"rootfs","size":31373893632,"mountpoint":"/","model":null,"vendor":null}]}
	]}`)
}

func (s *fakeStick) called(prefix string) bool {
	for _, c := range s.calls {
		if strings.HasPrefix(c, prefix) {
			return true
		}
	}

	return false
}

// plenty ist ein Stick mit reichlich Platz.
func plenty(string) (int64, error) { return 1 << 40, nil }

// photo legt ein Foto auf der "Fotobox" an.
func photo(t *testing.T, dir, name, content string, mtime time.Time) string {
	t.Helper()

	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.Chtimes(p, mtime, mtime); err != nil {
		t.Fatal(err)
	}

	return p
}

func TestDrivesListsOnlyTheStick(t *testing.T) {
	s := newStick(t)

	drives, err := usb.NewDrives(s)(user.SU(), context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(drives) != 1 || drives[0].Path != "/dev/sda1" || drives[0].Title() != "PARTY" || drives[0].SizeText() != "15,9 GB" {
		t.Fatalf("drives = %+v", drives)
	}

	want := "lsblk --json --bytes --output NAME,PATH,RM,HOTPLUG,TRAN,TYPE,FSTYPE,LABEL,SIZE,MOUNTPOINT,MODEL,VENDOR"
	if len(s.calls) != 1 || s.calls[0] != want {
		t.Fatalf("calls = %q", s.calls)
	}
}

func TestExportMountsOnDemandAndCopies(t *testing.T) {
	s := newStick(t)
	src := t.TempDir()
	taken := time.Date(2026, 9, 26, 21, 15, 0, 0, time.Local)

	a := photo(t, src, "1.jpg", "erstes", taken)
	b := photo(t, src, "2.jpg", "zweites Foto", taken)

	var progress []string

	report, err := usb.NewExport(s, plenty)(user.SU(), context.Background(), usb.ExportCmd{
		Drive:  "/dev/sda1",
		Folder: "eventprint/2026-09-26 Hochzeit Anna & Ben",
		Files:  []usb.File{{Name: "IMG_0001.jpg", Path: a}, {Name: "IMG_0002.jpg", Path: b}},
	}, func(done, total int) { progress = append(progress, fmt.Sprintf("%d/%d", done, total)) })
	if err != nil {
		t.Fatal(err)
	}

	if !s.called("udisksctl mount --no-user-interaction -b /dev/sda1") {
		t.Fatalf("der Stick wurde nicht eingehängt: %q", s.calls)
	}

	wantDir := filepath.Join(s.mount, "eventprint", "2026-09-26 Hochzeit Anna & Ben")
	if report.Folder != wantDir || report.Copied != 2 || report.Skipped != 0 || report.Bytes != int64(len("erstes")+len("zweites Foto")) {
		t.Fatalf("report = %+v", report)
	}

	if got, _ := os.ReadFile(filepath.Join(wantDir, "IMG_0002.jpg")); string(got) != "zweites Foto" {
		t.Fatalf("Inhalt = %q", got)
	}

	if st, err := os.Stat(filepath.Join(wantDir, "IMG_0001.jpg")); err != nil || !st.ModTime().Equal(taken) {
		t.Fatalf("Aufnahmedatum ging verloren: %v %v", st.ModTime(), err)
	}

	if strings.Join(progress, " ") != "0/2 1/2 2/2" {
		t.Fatalf("progress = %q", progress)
	}

	entries, _ := os.ReadDir(wantDir)
	if len(entries) != 2 {
		t.Fatalf("im Zielordner liegen Reste: %v", entries)
	}
}

func TestExportFindsMountPointViaLsblkWhenOutputIsUnknown(t *testing.T) {
	s := newStick(t)
	s.mountMsg = "Eingehängt, irgendwo.\n"

	a := photo(t, t.TempDir(), "1.jpg", "x", time.Now())

	report, err := usb.NewExport(s, plenty)(user.SU(), context.Background(), usb.ExportCmd{
		Drive: "/dev/sda1", Folder: "fotos", Files: []usb.File{{Name: "a.jpg", Path: a}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if report.Folder != filepath.Join(s.mount, "fotos") {
		t.Fatalf("Folder = %q", report.Folder)
	}
}

// TestExportIsIdempotent: Wird ein Export unterbrochen und wiederholt, darf
// kein Foto doppelt auf dem Stick landen.
func TestExportIsIdempotent(t *testing.T) {
	s := newStick(t)
	src := t.TempDir()
	cmd := usb.ExportCmd{
		Drive: "/dev/sda1", Folder: "eventprint/Feier",
		Files: []usb.File{
			{Name: "a.jpg", Path: photo(t, src, "a.jpg", "aaa", time.Now())},
			{Name: "b.jpg", Path: photo(t, src, "b.jpg", "bbbb", time.Now())},
		},
	}

	export := usb.NewExport(s, plenty)

	if _, err := export(user.SU(), context.Background(), cmd, nil); err != nil {
		t.Fatal(err)
	}

	report, err := export(user.SU(), context.Background(), cmd, nil)
	if err != nil {
		t.Fatal(err)
	}

	if report.Copied != 0 || report.Skipped != 2 || report.Bytes != 0 {
		t.Fatalf("report = %+v", report)
	}

	entries, _ := os.ReadDir(report.Folder)
	if len(entries) != 2 {
		t.Fatalf("Dateien = %v", entries)
	}
}

// TestExportNeverOverwrites: Auf dem Stick liegen womöglich Fotos einer
// früheren Feier mit demselben Namen.
func TestExportNeverOverwrites(t *testing.T) {
	s := newStick(t)
	dir := filepath.Join(s.mount, "fotos")
	photo(t, dir, "a.jpg", "alte Feier, anderes Bild", time.Now())
	photo(t, dir, "a (2).jpg", "noch eins", time.Now())

	src := t.TempDir()
	cmd := usb.ExportCmd{
		Drive: "/dev/sda1", Folder: "fotos",
		Files: []usb.File{
			{Name: "a.jpg", Path: photo(t, src, "1.jpg", "neu", time.Now())},
			// Zwei Fotos mit gleichem Wunschnamen im selben Export, in
			// anderer Schreibweise: FAT hielte sie für dieselbe Datei.
			{Name: "B.JPG", Path: photo(t, src, "2.jpg", "b eins", time.Now())},
			{Name: "b.jpg", Path: photo(t, src, "3.jpg", "b zwei!", time.Now())},
		},
	}

	export := usb.NewExport(s, plenty)

	report, err := export(user.SU(), context.Background(), cmd, nil)
	if err != nil {
		t.Fatal(err)
	}

	if report.Copied != 3 {
		t.Fatalf("report = %+v", report)
	}

	want := map[string]string{
		"a.jpg":     "alte Feier, anderes Bild",
		"a (2).jpg": "noch eins",
		"a (3).jpg": "neu",
		"B.JPG":     "b eins",
		"b (2).jpg": "b zwei!",
	}

	for name, content := range want {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || string(got) != content {
			t.Errorf("%s = %q, %v; want %q", name, got, err, content)
		}
	}

	// Die Wiederholung erkennt auch die umbenannten Fotos wieder.
	again, err := export(user.SU(), context.Background(), cmd, nil)
	if err != nil {
		t.Fatal(err)
	}

	if again.Copied != 0 || again.Skipped != 3 {
		t.Fatalf("Wiederholung = %+v", again)
	}
}

func TestExportFailsEarlyWhenTheStickIsFull(t *testing.T) {
	s := newStick(t)
	a := photo(t, t.TempDir(), "1.jpg", strings.Repeat("x", 4096), time.Now())

	full := func(string) (int64, error) { return 1000, nil }

	_, err := usb.NewExport(s, full)(user.SU(), context.Background(), usb.ExportCmd{
		Drive: "/dev/sda1", Folder: "fotos", Files: []usb.File{{Name: "a.jpg", Path: a}},
	}, nil)

	var space usb.NotEnoughSpaceError
	if !errors.As(err, &space) || space.Free != 1000 || !strings.Contains(err.Error(), "nicht genug Platz") {
		t.Fatalf("err = %v", err)
	}

	if _, err := os.Stat(filepath.Join(s.mount, "fotos")); !os.IsNotExist(err) {
		t.Fatal("trotz fehlenden Platzes wurde auf den Stick geschrieben")
	}
}

// TestExportCountsOnlyMissingFilesAgainstFreeSpace: Nach einem Abbruch ist
// der Stick fast voll mit genau den Bildern, die schon drauf sind.
func TestExportCountsOnlyMissingFilesAgainstFreeSpace(t *testing.T) {
	s := newStick(t)
	photo(t, filepath.Join(s.mount, "fotos"), "a.jpg", "schon da", time.Now())

	a := photo(t, t.TempDir(), "1.jpg", "schon da", time.Now())
	full := func(string) (int64, error) { return 0, nil }

	report, err := usb.NewExport(s, full)(user.SU(), context.Background(), usb.ExportCmd{
		Drive: "/dev/sda1", Folder: "fotos", Files: []usb.File{{Name: "a.jpg", Path: a}},
	}, nil)
	if err != nil || report.Skipped != 1 {
		t.Fatalf("report = %+v, err = %v", report, err)
	}
}

func TestExportRefusesTheSystemDisk(t *testing.T) {
	s := newStick(t)
	a := photo(t, t.TempDir(), "1.jpg", "x", time.Now())

	for _, drive := range []string{"/dev/mmcblk0p2", "/dev/mmcblk0", "", "/dev/sdz1"} {
		_, err := usb.NewExport(s, plenty)(user.SU(), context.Background(), usb.ExportCmd{
			Drive: drive, Folder: "x", Files: []usb.File{{Name: "a.jpg", Path: a}},
		}, nil)
		if err == nil {
			t.Errorf("%q wurde beschrieben", drive)
		}
	}

	if s.called("udisksctl") {
		t.Fatalf("udisksctl wurde aufgerufen: %q", s.calls)
	}
}

func TestExportStopsWhenCancelled(t *testing.T) {
	s := newStick(t)
	a := photo(t, t.TempDir(), "1.jpg", "x", time.Now())

	ctx, cancel := context.WithCancel(context.Background())
	export := usb.NewExport(s, plenty)

	// Erst einhängen, dann abbrechen: Der Abbruch soll das Kopieren
	// treffen, nicht schon die Laufwerksliste.
	s.mounted = true
	cancel()

	_, err := export(user.SU(), ctx, usb.ExportCmd{Drive: "/dev/sda1", Folder: "x", Files: []usb.File{{Name: "a.jpg", Path: a}}}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}

	entries, _ := os.ReadDir(filepath.Join(s.mount, "x"))
	if len(entries) != 0 {
		t.Fatalf("nach dem Abbruch liegen Reste: %v", entries)
	}
}

func TestSanitizeFolder(t *testing.T) {
	tests := []struct{ in, want string }{
		{"eventprint/2026-09-26 Hochzeit Anna & Ben", "eventprint/2026-09-26 Hochzeit Anna & Ben"},
		{"Sommerfest: Team \"Nord\" <2026>", "Sommerfest- Team _Nord_ _2026_"},
		{"../../etc", "etc"},
		{"a\\b|c*d?", "a-b-c_d_"},
		{" ..versteckt. / Feier.  ", "versteckt/Feier"},
		{"a\x00b\tc", "abc"},
		{"", "eventprint"},
		{"/../..", "eventprint"},
		{"nul", "_nul"},
		{strings.Repeat("x", 200) + "/y", strings.Repeat("x", 120) + "/y"},
	}

	for _, tt := range tests {
		if got := usb.SanitizeFolder(tt.in); got != tt.want {
			t.Errorf("SanitizeFolder(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestEjectUnmountsAndPowersOff(t *testing.T) {
	s := newStick(t)
	s.mounted = true

	if err := usb.NewEject(s)(user.SU(), context.Background(), "/dev/sda1"); err != nil {
		t.Fatal(err)
	}

	want := []string{
		"udisksctl unmount --no-user-interaction -b /dev/sda1",
		"udisksctl power-off --no-user-interaction -b /dev/sda",
	}

	var got []string
	for _, c := range s.calls {
		if strings.HasPrefix(c, "udisksctl") {
			got = append(got, c)
		}
	}

	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("calls = %q", got)
	}
}

func TestEject(t *testing.T) {
	tests := []struct {
		name      string
		mounted   bool
		drive     string
		unmountEr error
		unmountTx string
		powerErr  error
		wantBusy  bool
		wantErr   bool
	}{
		{name: "nicht eingehängt, nur abschalten", drive: "/dev/sda1"},
		{name: "Abschalten scheitert, trotzdem gut", mounted: true, drive: "/dev/sda1", powerErr: errors.New("kann nicht")},
		{name: "schon abgezogen", drive: "/dev/sdz1"},
		{
			name: "noch in Benutzung", mounted: true, drive: "/dev/sda1",
			unmountEr: errors.New("exit status 1"),
			unmountTx: "Error unmounting /dev/sda1: GDBus.Error:org.freedesktop.UDisks2.Error.DeviceBusy: Error unmounting /dev/sda1: target is busy",
			wantBusy:  true, wantErr: true,
		},
		{
			name: "zwischendurch ausgehängt", mounted: true, drive: "/dev/sda1",
			unmountEr: errors.New("exit status 1"),
			unmountTx: "Error unmounting /dev/sda1: GDBus.Error:org.freedesktop.UDisks2.Error.NotMounted: Device `/dev/sda1' is not mounted",
		},
		{
			name: "anderer Fehler", mounted: true, drive: "/dev/sda1",
			unmountEr: errors.New("exit status 1"), unmountTx: "Error: NotAuthorized", wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newStick(t)
			s.mounted = tt.mounted
			s.unmountErr = tt.unmountEr
			s.unmountOut = tt.unmountTx
			s.powerErr = tt.powerErr

			err := usb.NewEject(s)(user.SU(), context.Background(), tt.drive)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v", err)
			}

			var busy usb.BusyError
			if errors.As(err, &busy) != tt.wantBusy {
				t.Fatalf("err = %v, busy erwartet: %v", err, tt.wantBusy)
			}

			if tt.wantBusy && !strings.Contains(err.Error(), "warten") {
				t.Fatalf("die Meldung sagt nicht, was zu tun ist: %v", err)
			}

			if tt.wantBusy && s.called("udisksctl power-off") {
				t.Fatal("ein belegter Stick wurde abgeschaltet")
			}
		})
	}
}

func TestImagesFindsPrintableImagesNewestFirst(t *testing.T) {
	s := newStick(t)
	root := s.mount
	base := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)

	photo(t, root, "DCIM/100CANON/IMG_0001.JPG", "a", base)
	photo(t, root, "DCIM/100CANON/IMG_0002.jpeg", "bb", base.Add(2*time.Hour))
	photo(t, root, "urlaub.png", "ccc", base.Add(time.Hour))
	photo(t, root, "a/b/c/d/e/f/tief.jpg", "6 Ebenen", base)

	// All das darf nicht erscheinen.
	photo(t, root, "a/b/c/d/e/f/g/zu tief.jpg", "7 Ebenen", base)
	photo(t, root, "notizen.txt", "x", base)
	photo(t, root, "iphone.heic", "x", base)
	photo(t, root, "DCIM/100CANON/._IMG_0001.JPG", "AppleDouble", base)
	photo(t, root, ".versteckt/x.jpg", "x", base)
	photo(t, root, "System Volume Information/x.jpg", "x", base)
	photo(t, root, "$RECYCLE.BIN/x.jpg", "x", base)
	photo(t, root, ".Trashes/501/x.jpg", "x", base)
	photo(t, root, ".Spotlight-V100/x.jpg", "x", base)

	images, err := usb.NewImages(s)(user.SU(), context.Background(), "/dev/sda1")
	if err != nil {
		t.Fatal(err)
	}

	var got []string
	for _, img := range images {
		rel, _ := filepath.Rel(root, img.Path)
		got = append(got, filepath.ToSlash(rel))
	}

	want := []string{"DCIM/100CANON/IMG_0002.jpeg", "urlaub.png", "DCIM/100CANON/IMG_0001.JPG", "a/b/c/d/e/f/tief.jpg"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	if images[0].Name != "IMG_0002.jpeg" || images[0].Size != 2 || !images[0].ModTime.Equal(base.Add(2*time.Hour)) {
		t.Fatalf("images[0] = %+v", images[0])
	}

	if !s.called("udisksctl mount") {
		t.Fatal("der Stick wurde nicht eingehängt")
	}
}

func TestReadStaysOnTheStick(t *testing.T) {
	s := newStick(t)
	s.mounted = true

	inside := photo(t, s.mount, "DCIM/foto.jpg", "JPEG", time.Now())
	outside := photo(t, t.TempDir(), "geheim.jpg", "geheim", time.Now())
	notImage := photo(t, s.mount, "notizen.txt", "x", time.Now())

	link := filepath.Join(s.mount, "link.jpg")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}

	big := filepath.Join(s.mount, "riesig.jpg")
	if f, err := os.Create(big); err != nil {
		t.Fatal(err)
	} else {
		_ = f.Truncate(61 << 20)
		_ = f.Close()
	}

	read := usb.NewRead(s)

	data, err := read(user.SU(), context.Background(), inside)
	if err != nil || !bytes.Equal(data, []byte("JPEG")) {
		t.Fatalf("data = %q, err = %v", data, err)
	}

	for name, path := range map[string]string{
		"außerhalb":        outside,
		"Verknüpfung raus": link,
		"über ..":          filepath.Join(s.mount, "DCIM", "..", "..", filepath.Base(filepath.Dir(outside)), "geheim.jpg"),
		"kein Bild":        notImage,
		"zu groß":          big,
		"relativ":          "DCIM/foto.jpg",
		"Gerätedatei":      "/dev/sda1",
		"fehlt":            filepath.Join(s.mount, "fehlt.jpg"),
	} {
		if _, err := read(user.SU(), context.Background(), path); err == nil {
			t.Errorf("%s: %s wurde gelesen", name, path)
		}
	}

	// Ausgehängt gibt es keinen Einhängepunkt mehr, dem Read trauen dürfte.
	s.mounted = false
	if _, err := read(user.SU(), context.Background(), inside); err == nil {
		t.Error("ein Bild eines nicht eingehängten Sticks wurde gelesen")
	}
}

// TestGuestMayDoNothing hält fest, dass jede Berechtigung greift, bevor
// irgendein Programm läuft: Über den Stick verlassen alle Fotos das Gerät.
func TestGuestMayDoNothing(t *testing.T) {
	s := newStick(t)
	s.mounted = true
	ctx := context.Background()
	uc := usb.NewUseCases(s, plenty)

	tests := map[string]func() error{
		"Drives": func() error { _, err := uc.Drives(guest{}, ctx); return err },
		"Export": func() error {
			_, err := uc.Export(guest{}, ctx, usb.ExportCmd{Drive: "/dev/sda1", Files: []usb.File{{Name: "a.jpg", Path: "/etc/hosts"}}}, nil)
			return err
		},
		"Eject":  func() error { return uc.Eject(guest{}, ctx, "/dev/sda1") },
		"Images": func() error { _, err := uc.Images(guest{}, ctx, "/dev/sda1"); return err },
		"Read":   func() error { _, err := uc.Read(guest{}, ctx, filepath.Join(s.mount, "a.jpg")); return err },
	}

	for name, call := range tests {
		if err := call(); err == nil {
			t.Errorf("%s: ein Gast wurde durchgelassen", name)
		}
	}

	if len(s.calls) != 0 {
		t.Fatalf("trotz fehlender Berechtigung aufgerufen: %q", s.calls)
	}

	if len(usb.Permissions()) != 5 {
		t.Fatalf("Permissions = %v", usb.Permissions())
	}
}
