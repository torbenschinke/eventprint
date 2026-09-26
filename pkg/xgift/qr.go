package xgift

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"sync"

	"rsc.io/qr"

	"github.com/worldiety/gift"
	"github.com/worldiety/gift/ui"
)

var qrCache sync.Map // string -> *MemorySource

// QRSource renders text as a QR code picture with a quiet zone, or nil if the
// text does not fit into a QR code.
//
// The picture is rendered with a whole number of pixels per module so that
// scaling it down in the image pipeline keeps the edges crisp enough for a
// phone camera. Results are cached by text; a screen that redraws its code
// every second does not encode it every second.
func QRSource(text string) *MemorySource {
	if v, ok := qrCache.Load(text); ok {
		return v.(*MemorySource)
	}

	code, err := qr.Encode(text, qr.M)
	if err != nil {
		return nil
	}

	const quiet, scale = 4, 12
	size := (code.Size + 2*quiet) * scale
	img := image.NewGray(image.Rect(0, 0, size, size))
	for i := range img.Pix {
		img.Pix[i] = 0xFF
	}

	for y := range code.Size {
		for x := range code.Size {
			if !code.Black(x, y) {
				continue
			}

			for dy := range scale {
				for dx := range scale {
					img.SetGray((x+quiet)*scale+dx, (y+quiet)*scale+dy, color.Gray{})
				}
			}
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}

	src := Memory(buf.Bytes())
	qrCache.Store(text, src)

	return src
}

// QRCode shows text as a square QR code of the given edge length.
func QRCode(text string, edge float32) gift.View {
	src := QRSource(text)
	if src == nil {
		return ui.Box().Frame(edge, edge).Background(ui.ColorSeparator).CornerRadius(edge / 16)
	}

	return ui.Image(src).
		Fit(ui.FitContain).
		Frame(edge, edge).
		Background(ui.OpaqueWhite).
		CornerRadius(edge / 16).
		Clip(true).
		Label("QR-Code")
}
