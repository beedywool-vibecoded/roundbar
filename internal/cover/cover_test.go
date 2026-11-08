package cover

import (
	"context"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"os"
	"strconv"
	"testing"
)

// ROUNDBAR_ART=file:///… ROUNDBAR_OUT=dir go test ./internal/cover -run Preview -v
func TestPreview(t *testing.T) {
	art, out := os.Getenv("ROUNDBAR_ART"), os.Getenv("ROUNDBAR_OUT")
	if art == "" || out == "" {
		t.Skip("set ROUNDBAR_ART and ROUNDBAR_OUT")
	}
	img, err := Load(context.Background(), art)
	if err != nil {
		t.Fatal(err)
	}
	raw := Blurred(img)
	bg := image.NewRGBA(image.Rect(0, 0, Size+200, Size))
	for i := 0; i < Size*Size; i++ {
		v := binary.LittleEndian.Uint16(raw[2*i:])
		bg.Set(i%Size, i/Size, color.RGBA{uint8(v>>11) << 3, uint8(v>>5&63) << 2, uint8(v&31) << 3, 255})
	}
	pal := Palette(img, true, map[string]string{})
	keys := []string{"surface", "primary", "primaryContainer", "secondaryContainer", "tertiary", "tertiaryContainer", "onSurface"}
	for k, name := range keys {
		h := pal[name]
		r, _ := strconv.ParseUint(h[1:3], 16, 8)
		g, _ := strconv.ParseUint(h[3:5], 16, 8)
		b, _ := strconv.ParseUint(h[5:7], 16, 8)
		for y := k * 50; y < k*50+46; y++ {
			for x := Size + 10; x < Size+190; x++ {
				bg.Set(x, y, color.RGBA{uint8(r), uint8(g), uint8(b), 255})
			}
		}
	}
	t.Logf("%v", pal)
	f, _ := os.Create(out + "/preview.png")
	png.Encode(f, bg)
	f.Close()
}

func TestGray(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 64, 64))
	p := Palette(img, true, map[string]string{})
	if len(p) < 17 || p["primary"] == "" {
		t.Fatal(p)
	}
	if len(Blurred(img)) != Size*Size*2 {
		t.Fatal("size")
	}
}
