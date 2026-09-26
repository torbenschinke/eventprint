package main

import (
	"testing"

	"github.com/worldiety/gift/geom"
)

func TestParseXrandr(t *testing.T) {
	out := `Screen 0: minimum 320 x 200, current 1280 x 720, maximum 7680 x 7680
HDMI-1 connected 1280x720+0+0 (normal left inverted right x axis y axis) 1600mm x 900mm
   1920x1080     60.00 +
DSI-1 connected primary 1280x720+0+0 right (normal left inverted right x axis y axis) 88mm x 155mm
   720x1280      60.00*+
HDMI-2 disconnected (normal left inverted right x axis y axis)
`
	if got, ok := parseXrandr(out); !ok || got != geom.Sz(88, 155) {
		t.Fatalf("primary = %v, %v", got, ok)
	}

	if got, ok := parseXrandr("HDMI-1 connected 1024x600+0+0 (normal) 154mm x 86mm\n"); !ok || got != geom.Sz(154, 86) {
		t.Fatalf("first = %v, %v", got, ok)
	}

	if _, ok := parseXrandr("HDMI-1 connected 1024x600+0+0 (normal) 0mm x 0mm\n"); ok {
		t.Fatal("zero size must be unknown")
	}
}
