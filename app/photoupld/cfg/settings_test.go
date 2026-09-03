package photoupld

import "testing"

func TestUploadURL(t *testing.T) {
	tests := []struct {
		name     string
		settings Settings
		fallback string
		want     string
	}{
		{name: "configured", settings: Settings{PublicURL: "https://upload.example.de/base/"}, want: "https://upload.example.de/base/upload?u=abc"},
		{name: "adds scheme", settings: Settings{PublicURL: "upload.example.de"}, want: "https://upload.example.de/upload?u=abc"},
		{name: "fallback", fallback: "http://localhost:3000", want: "http://localhost:3000/upload?u=abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.settings.UploadURL("abc", func() string { return tt.fallback })
			if got != tt.want {
				t.Fatalf("UploadURL = %q, want %q", got, tt.want)
			}
		})
	}
}
