// winicon renders the app icon (cmd/roundbar/icon.svg, the configurator's
// favicon) into a Windows .ico for the installer, its shortcuts and Apps &
// features: 16 to 48 px as 32-bit bitmaps, 256 px as PNG.
//
//	go run ./tools/winicon -o deploy/windows/roundbar.ico
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"image"
	"image/png"
	"log"
	"os"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

func render(svg []byte, size int) *image.NRGBA {
	icon, err := oksvg.ReadIconStream(bytes.NewReader(svg), oksvg.IgnoreErrorMode)
	if err != nil {
		log.Fatal(err)
	}
	icon.SetTarget(0, 0, float64(size), float64(size))
	rgba := image.NewRGBA(image.Rect(0, 0, size, size))
	icon.Draw(rasterx.NewDasher(size, size, rasterx.NewScannerGV(size, size, rgba, rgba.Bounds())), 1)
	out := image.NewNRGBA(rgba.Rect)
	for i := 0; i < size*size; i++ {
		r, g, b, a := int(rgba.Pix[4*i]), int(rgba.Pix[4*i+1]), int(rgba.Pix[4*i+2]), int(rgba.Pix[4*i+3])
		if a > 0 && a < 255 {
			r, g, b = r*255/a, g*255/a, b*255/a
		}
		out.Pix[4*i], out.Pix[4*i+1], out.Pix[4*i+2], out.Pix[4*i+3] = byte(r), byte(g), byte(b), byte(a)
	}
	return out
}

// dib is an icon image in the classic format: BITMAPINFOHEADER (double
// height), bottom-up BGRA, then the 1-bit AND mask.
func dib(img *image.NRGBA) []byte {
	n := img.Rect.Dx()
	var b bytes.Buffer
	hdr := struct {
		Size                  uint32
		Width, Height         int32
		Planes, BitCount      uint16
		Compression, SizeImg  uint32
		XPels, YPels          int32
		ClrUsed, ClrImportant uint32
	}{Size: 40, Width: int32(n), Height: int32(2 * n), Planes: 1, BitCount: 32}
	_ = binary.Write(&b, binary.LittleEndian, hdr)
	for y := n - 1; y >= 0; y-- {
		for x := 0; x < n; x++ {
			p := img.Pix[img.PixOffset(x, y):]
			b.Write([]byte{p[2], p[1], p[0], p[3]})
		}
	}
	stride := (n + 31) / 32 * 4
	for y := n - 1; y >= 0; y-- {
		row := make([]byte, stride)
		for x := 0; x < n; x++ {
			if img.Pix[img.PixOffset(x, y)+3] == 0 {
				row[x/8] |= 0x80 >> (x % 8)
			}
		}
		b.Write(row)
	}
	return b.Bytes()
}

func main() {
	in := flag.String("i", "cmd/roundbar/icon.svg", "SVG to render")
	out := flag.String("o", "roundbar.ico", "output .ico")
	flag.Parse()
	svg, err := os.ReadFile(*in)
	if err != nil {
		log.Fatal(err)
	}
	sizes := []int{16, 20, 24, 32, 40, 48, 64, 256}
	var images [][]byte
	for _, s := range sizes {
		img := render(svg, s)
		if s >= 256 {
			var p bytes.Buffer
			if err := png.Encode(&p, img); err != nil {
				log.Fatal(err)
			}
			images = append(images, p.Bytes())
		} else {
			images = append(images, dib(img))
		}
	}
	var f bytes.Buffer
	_ = binary.Write(&f, binary.LittleEndian, [3]uint16{0, 1, uint16(len(sizes))}) // ICONDIR
	offset := 6 + 16*len(sizes)
	for i, s := range sizes {
		dim := byte(s)
		if s >= 256 {
			dim = 0
		}
		_ = binary.Write(&f, binary.LittleEndian, struct {
			W, H, Colors, Reserved byte
			Planes, BitCount       uint16
			Bytes, Offset          uint32
		}{dim, dim, 0, 0, 1, 32, uint32(len(images[i])), uint32(offset)})
		offset += len(images[i])
	}
	for _, img := range images {
		f.Write(img)
	}
	if err := os.WriteFile(*out, f.Bytes(), 0o644); err != nil {
		log.Fatal(err)
	}
}
