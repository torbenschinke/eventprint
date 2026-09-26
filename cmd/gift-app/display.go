package main

import (
	"context"
	"os/exec"
	"regexp"
	"strconv"
	"time"

	"github.com/worldiety/gift/geom"
)

// physicalSize fragt xrandr nach der Größe des führenden Panels in
// Millimetern. Ohne X11 oder ohne Angabe bleibt es bei der Bemessung nach der
// Auflösung.
func physicalSize() (geom.Size, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "xrandr", "--query").Output()
	if err != nil {
		return geom.Size{}, false
	}

	return parseXrandr(string(out))
}

// connectedLine findet Ausgänge mit Maßangabe, etwa
//
//	DSI-1 connected primary 1280x720+0+0 right (normal left inverted right x axis y axis) 155mm x 88mm
var connectedLine = regexp.MustCompile(`(?m)^\S+ connected( primary)? \d+x\d+\+\d+\+\d+.* (\d+)mm x (\d+)mm`)

// parseXrandr nimmt den primären Ausgang, sonst den ersten mit Maßen. Das ist
// der Touchscreen: Die Anzeigesitzung macht ihn zum primären, und ein
// gespiegelter Fernseher hat dieselbe Auflösung, aber nicht dieselben Maße.
func parseXrandr(out string) (geom.Size, bool) {
	var first geom.Size
	for _, m := range connectedLine.FindAllStringSubmatch(out, -1) {
		w, _ := strconv.Atoi(m[2])
		h, _ := strconv.Atoi(m[3])
		if w <= 0 || h <= 0 {
			continue
		}

		sz := geom.Sz(float32(w), float32(h))
		if m[1] != "" {
			return sz, true
		}

		if first.W == 0 {
			first = sz
		}
	}

	return first, first.W > 0
}

// parseSize liest "800x480".
func parseSize(s string) (int, int, bool) {
	m := regexp.MustCompile(`^(\d+)x(\d+)$`).FindStringSubmatch(s)
	if m == nil {
		return 0, 0, false
	}

	w, _ := strconv.Atoi(m[1])
	h, _ := strconv.Atoi(m[2])

	return w, h, w > 0 && h > 0
}
