package assets

import (
	"bytes"
	"image"
	"image/color"
	"image/color/palette"
	"image/gif"
	"image/jpeg"
	"testing"
)

func TestStillAndGIF(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	src := image.NewRGBA(image.Rect(0, 0, 1600, 900))
	for i := range src.Pix {
		src.Pix[i] = byte(i)
	}
	var buf bytes.Buffer
	jpeg.Encode(&buf, src, nil)
	info, err := Store(buf.Bytes())
	if err != nil || info.Kind != "image" || info.W != 1080 {
		t.Fatalf("%+v %v", info, err)
	}
	for _, fit := range []string{"cover", "contain"} {
		b, err := DeviceFile(info.ID, fit)
		if err != nil {
			t.Fatal(err)
		}
		cfg, _ := jpeg.DecodeConfig(bytes.NewReader(b))
		t.Logf("%s: %dx%d %d bytes", fit, cfg.Width, cfg.Height, len(b))
		if fit == "cover" && (cfg.Width != 360 || cfg.Height != 360) {
			t.Fatal("cover must be 360x360")
		}
		if fit == "contain" && (cfg.Width != 360 || cfg.Height != 202) {
			t.Fatalf("contain got %dx%d", cfg.Width, cfg.Height)
		}
	}
	g := &gif.GIF{}
	for k := 0; k < 3; k++ {
		f := image.NewPaletted(image.Rect(0, 0, 100, 50), palette.Plan9)
		f.Set(k, k, color.White)
		g.Image = append(g.Image, f)
		g.Delay = append(g.Delay, 5)
	}
	buf.Reset()
	gif.EncodeAll(&buf, g)
	gi, err := Store(buf.Bytes())
	if err != nil || gi.Kind != "gif" {
		t.Fatal(err)
	}
	b, err := DeviceFile(gi.ID, "cover")
	if err != nil {
		t.Fatal(err)
	}
	out, _ := gif.DecodeAll(bytes.NewReader(b))
	if len(out.Image) != 3 || out.Image[0].Bounds().Dx() != 360 {
		t.Fatal("gif frames")
	}
	if _, err := Store([]byte("hello")); err != ErrNotImage {
		t.Fatal("expected ErrNotImage")
	}
}
