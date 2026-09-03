package printing

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image/jpeg"
	"io"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
)

const (
	cupsRasterHeaderSize = 1796
	cupsRasterMagicSize  = 4
	cupsRasterMagic      = "3SaR"
)

type cupsRasterHeader struct {
	Width        uint32
	Height       uint32
	DPIX         uint32
	DPIY         uint32
	BitsPerColor uint32
	BitsPerPixel uint32
	BytesPerLine uint32
	ColorOrder   uint32
	ColorSpace   uint32
	NumColors    uint32
}

func validateCZ01PPD(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cannot read CUPS PPD %s: %w", path, err)
	}
	text := string(data)

	areaPattern := regexp.MustCompile(`(?m)^\*ImageableArea w288h432/[^:]+:\s*"([0-9.]+) ([0-9.]+) ([0-9.]+) ([0-9.]+)"`)
	match := areaPattern.FindStringSubmatch(text)
	if match == nil {
		return fmt.Errorf("PPD %s has no ImageableArea for %s", path, CupsPageSize)
	}
	values := make([]float64, 4)
	for i := range values {
		values[i], err = strconv.ParseFloat(match[i+1], 64)
		if err != nil {
			return fmt.Errorf("invalid ImageableArea in PPD %s: %w", path, err)
		}
	}
	width := int((values[2]-values[0])*DPI/72 + 0.5)
	height := int((values[3]-values[1])*DPI/72 + 0.5)
	if width != NativeRaster4x6.Width || height != NativeRaster4x6.Height {
		return fmt.Errorf("PPD %s declares %dx%d for %s, expected %dx%d",
			path, width, height, CupsPageSize, NativeRaster4x6.Width, NativeRaster4x6.Height)
	}
	if !strings.Contains(text, `*Resolution 300dpi/`) {
		return fmt.Errorf("PPD %s does not declare 300dpi", path)
	}
	return nil
}

func writeCZ01Raster(dst io.Writer, jpegData []byte) error {
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(jpegData))
	if err != nil {
		return fmt.Errorf("cannot read print JPEG dimensions: %w", err)
	}
	if !NativeRaster4x6.Matches(cfg.Width, cfg.Height) {
		return fmt.Errorf("invalid CZ-01 print image: got %dx%d, expected %dx%d or %dx%d",
			cfg.Width, cfg.Height,
			NativeRaster4x6.Width, NativeRaster4x6.Height,
			NativeRaster4x6.Height, NativeRaster4x6.Width)
	}

	img, err := jpeg.Decode(bytes.NewReader(jpegData))
	if err != nil {
		return fmt.Errorf("cannot decode print JPEG: %w", err)
	}

	header := make([]byte, cupsRasterHeaderSize)
	putU32 := func(offset int, value uint32) { binary.LittleEndian.PutUint32(header[offset:], value) }
	putF32 := func(offset int, value float32) {
		binary.LittleEndian.PutUint32(header[offset:], math.Float32bits(value))
	}

	putU32(276, DPI)
	putU32(280, DPI)
	putU32(284, 17)
	putU32(288, 0)
	putU32(292, 321)
	putU32(296, 441)
	putU32(312, 17)
	putU32(316, 0)
	putU32(352, 338)
	putU32(356, 441)
	putU32(372, uint32(NativeRaster4x6.Width))
	putU32(376, uint32(NativeRaster4x6.Height))
	putU32(384, 8)
	putU32(388, 24)
	putU32(392, uint32(NativeRaster4x6.Width*3))
	putU32(396, 0) // CUPS_ORDER_CHUNKED
	putU32(400, 1) // CUPS_CSPACE_RGB
	putU32(404, 1)
	putU32(420, 3)
	putF32(428, 337.92)
	putF32(432, 440.64)
	putF32(436, 17.04)
	putF32(440, 0)
	putF32(444, 320.88)
	putF32(448, 440.64)
	copy(header[1732:], CupsPageSize)

	if _, err := io.WriteString(dst, cupsRasterMagic); err != nil {
		return err
	}
	if _, err := dst.Write(header); err != nil {
		return err
	}

	landscape := cfg.Width > cfg.Height
	row := make([]byte, NativeRaster4x6.Width*3)
	for y := range NativeRaster4x6.Height {
		for x := range NativeRaster4x6.Width {
			sx, sy := x, y
			if landscape {
				// Match CUPS' landscape orientation: rotate 90 degrees CCW.
				sx, sy = cfg.Width-1-y, x
			}
			r, g, b, _ := img.At(img.Bounds().Min.X+sx, img.Bounds().Min.Y+sy).RGBA()
			off := x * 3
			row[off+0] = byte(r >> 8)
			row[off+1] = byte(g >> 8)
			row[off+2] = byte(b >> 8)
		}
		if _, err := dst.Write(row); err != nil {
			return err
		}
	}

	return nil
}

func validateCZ01Raster(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("cannot open CUPS raster: %w", err)
	}
	defer f.Close()

	header, err := readCUPSRasterHeader(f)
	if err != nil {
		return err
	}
	if header.Width != uint32(NativeRaster4x6.Width) || header.Height != uint32(NativeRaster4x6.Height) ||
		header.DPIX != DPI || header.DPIY != DPI || header.BitsPerColor != 8 ||
		header.BitsPerPixel != 24 || header.BytesPerLine != uint32(NativeRaster4x6.Width*3) ||
		header.ColorOrder != 0 || header.ColorSpace != 1 || header.NumColors != 3 {
		return fmt.Errorf("invalid CZ-01 CUPS raster header: %+v", header)
	}

	info, err := f.Stat()
	if err != nil {
		return err
	}
	wantSize := int64(cupsRasterMagicSize+cupsRasterHeaderSize) + int64(header.BytesPerLine*header.Height)
	if info.Size() != wantSize {
		return fmt.Errorf("invalid CZ-01 CUPS raster length: got %d, expected %d", info.Size(), wantSize)
	}
	return nil
}

func readCUPSRasterHeader(r io.Reader) (cupsRasterHeader, error) {
	buf := make([]byte, cupsRasterMagicSize+cupsRasterHeaderSize)
	if _, err := io.ReadFull(r, buf); err != nil {
		return cupsRasterHeader{}, fmt.Errorf("cannot read CUPS raster header: %w", err)
	}
	if string(buf[:cupsRasterMagicSize]) != cupsRasterMagic {
		return cupsRasterHeader{}, fmt.Errorf("invalid CUPS raster magic %q", buf[:cupsRasterMagicSize])
	}
	h := buf[cupsRasterMagicSize:]
	u32 := func(offset int) uint32 { return binary.LittleEndian.Uint32(h[offset:]) }
	return cupsRasterHeader{
		Width: u32(372), Height: u32(376), DPIX: u32(276), DPIY: u32(280),
		BitsPerColor: u32(384), BitsPerPixel: u32(388), BytesPerLine: u32(392),
		ColorOrder: u32(396), ColorSpace: u32(400), NumColors: u32(420),
	}, nil
}

func validateCZ01PrintStream(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cannot read print stream: %w", err)
	}
	for _, plane := range []string{"YPLANE", "MPLANE", "CPLANE"} {
		marker := []byte("\x1bPIMAGE " + plane)
		i := bytes.Index(data, marker)
		if i < 0 || i+32 > len(data) {
			return fmt.Errorf("missing %s command", plane)
		}
		length, err := strconv.Atoi(string(data[i+24 : i+32]))
		if err != nil || length < 54 || i+32+length > len(data) {
			return fmt.Errorf("invalid %s payload length", plane)
		}
		bmp := data[i+32 : i+32+length]
		if string(bmp[:2]) != "BM" {
			return fmt.Errorf("%s payload is not BMP", plane)
		}
		width := int(int32(binary.LittleEndian.Uint32(bmp[18:22])))
		height := int(int32(binary.LittleEndian.Uint32(bmp[22:26])))
		bits := binary.LittleEndian.Uint16(bmp[28:30])
		if width != 1408 || height != NativeRaster4x6.Height || bits != 8 {
			return fmt.Errorf("%s is %dx%d@%dbpp, expected 1408x%d@8bpp",
				plane, width, height, bits, NativeRaster4x6.Height)
		}
	}
	return nil
}
