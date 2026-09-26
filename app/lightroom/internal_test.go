package lightroom

import (
	"testing"
	"time"
)

func TestStripJSONPrefix(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"while (1) {}\n{\"id\":1}", `{"id":1}`},
		{"while (1) {}{\"id\":1}", `{"id":1}`},
		{"while(1){}\r\n  {\"id\":1}", `{"id":1}`},
		{"  {\"id\":1}  ", `{"id":1}`},
		{"[1,2]", `[1,2]`},
	}

	for _, tt := range tests {
		if got := string(stripJSONPrefix([]byte(tt.in))); got != tt.want {
			t.Errorf("stripJSONPrefix(%q) = %q, erwartet %q", tt.in, got, tt.want)
		}
	}
}

func TestNextCursor(t *testing.T) {
	cfg := Config{APIBaseURL: "https://lr.adobe.io"}.normalized()
	requested := "https://lr.adobe.io/v2/catalogs/c1/albums/a1/assets?embed=asset&limit=100"

	tests := []struct {
		name, base, href, want string
		wantErr                bool
	}{
		{name: "ohne Verweis"},
		{name: "relativ zur base", base: "https://lr.adobe.io/v2/catalogs/c1/", href: "albums/a1/assets?after=x",
			want: "/v2/catalogs/c1/albums/a1/assets?after=x"},
		{name: "absoluter Pfad", base: "https://lr.adobe.io/v2/catalogs/c1/", href: "/v2/catalogs/c1/assets?after=y",
			want: "/v2/catalogs/c1/assets?after=y"},
		{name: "relativ zur Anfrage", href: "assets?after=z",
			want: "/v2/catalogs/c1/albums/a1/assets?after=z"},
		{name: "fremder Rechner", href: "https://evil.example/v2/x", wantErr: true},
		{name: "fremdes Schema", href: "http://lr.adobe.io/v2/x", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var env listEnvelope
			env.Base = tt.base

			if tt.href != "" {
				env.Links.Next = &struct {
					Href string `json:"href"`
				}{tt.href}
			}

			got, err := nextCursor(cfg, requested, env)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v", err)
			}

			if got != tt.want {
				t.Fatalf("Cursor = %q, erwartet %q", got, tt.want)
			}
		})
	}
}

func TestParseCaptureDate(t *testing.T) {
	tests := []struct {
		in   string
		want time.Time
	}{
		{"", time.Time{}},
		{"0000-00-00T00:00:00", time.Time{}},
		{"unsinn", time.Time{}},
		{"2024-06-01T14:30:00Z", time.Date(2024, 6, 1, 14, 30, 0, 0, time.UTC)},
		{"2024-06-01T14:30:00", time.Date(2024, 6, 1, 14, 30, 0, 0, time.Local)},
		{"2024-06-01T14:30:00.5", time.Date(2024, 6, 1, 14, 30, 0, 500_000_000, time.Local)},
		{"2024-06-01", time.Date(2024, 6, 1, 0, 0, 0, 0, time.Local)},
	}

	for _, tt := range tests {
		if got := parseCaptureDate(tt.in); !got.Equal(tt.want) {
			t.Errorf("parseCaptureDate(%q) = %v, erwartet %v", tt.in, got, tt.want)
		}
	}
}

func TestArchiveName(t *testing.T) {
	tests := []struct {
		file, id, want string
	}{
		{"IMG_0001.CR3", "x", "IMG_0001.jpg"},
		{"urlaub.jpeg", "x", "urlaub.jpg"},
		{"../../etc/passwd", "x", "passwd.jpg"},
		{`C:\a\b.dng`, "x", "b.jpg"},
		{"", "ab/../12", "lightroom-ab12.jpg"},
		{".hidden", "id1", "lightroom-id1.jpg"},
	}

	for _, tt := range tests {
		if got := archiveName(tt.file, tt.id); got != tt.want {
			t.Errorf("archiveName(%q, %q) = %q, erwartet %q", tt.file, tt.id, got, tt.want)
		}
	}
}

func TestStatusString(t *testing.T) {
	for s, want := range map[Status]string{StatusPending: "pending", StatusConnected: "connected", StatusExpired: "expired", Status(9): "unknown"} {
		if s.String() != want {
			t.Errorf("%d.String() = %q", s, s.String())
		}
	}
}
