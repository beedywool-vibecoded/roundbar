// Package assets stores images uploaded in the configurator and renders the
// 360×360 versions the device shows (docs/spec.md §1, "Assets").
package assets

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/color/palette"
	"image/draw"
	"image/gif"
	"image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/beedywool-vibecoded/roundbar/internal/config"
	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const (
	Screen      = 360
	maxStored   = 1080
	maxUpload   = 20 << 20
	maxDeviceGI = 3 << 20
)

var ErrNotImage = errors.New("not an image")

func Dir() string { return filepath.Join(config.Dir(), "assets") }

func Path(id string) string { return filepath.Join(Dir(), filepath.Base(id)) }

type Info struct {
	ID   string `json:"id"`
	Kind string `json:"kind"` // image | gif
	W    int    `json:"w"`
	H    int    `json:"h"`
	Size int    `json:"size"`
}

func idFor(data []byte, ext string) string {
	s := sha256.Sum256(data)
	return hex.EncodeToString(s[:6]) + ext
}

// Store converts an upload and writes it under Dir(). Stills become JPEG with
// the longest side ≤ 1080 px; GIFs are kept byte for byte.
func Store(raw []byte) (Info, error) {
	if len(raw) > maxUpload {
		return Info{}, fmt.Errorf("image too large (max %d MB)", maxUpload>>20)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return Info{}, ErrNotImage
	}
	var data []byte
	info := Info{Kind: "image", W: cfg.Width, H: cfg.Height}
	if format == "gif" {
		info.Kind = "gif"
		data = raw
		info.ID = idFor(data, ".gif")
	} else {
		img, _, err := image.Decode(bytes.NewReader(raw))
		if err != nil {
			return Info{}, ErrNotImage
		}
		b := img.Bounds()
		if s := max(b.Dx(), b.Dy()); s > maxStored {
			img = scale(img, b.Dx()*maxStored/s, b.Dy()*maxStored/s)
		}
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, flatten(img), &jpeg.Options{Quality: 90}); err != nil {
			return Info{}, err
		}
		data = buf.Bytes()
		info.ID = idFor(data, ".jpg")
		info.W, info.H = img.Bounds().Dx(), img.Bounds().Dy()
	}
	info.Size = len(data)
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		return Info{}, err
	}
	return info, os.WriteFile(Path(info.ID), data, 0o644)
}

// DeviceName is the file name the device stores for an asset shown with fit.
func DeviceName(id, fit string) string {
	ext := filepath.Ext(id)
	f := "c"
	if fit == "contain" {
		f = "f"
	}
	return strings.TrimSuffix(id, ext) + "-" + f + ext
}

// DeviceFile renders (and caches on disk) the device version of an asset.
func DeviceFile(id, fit string) ([]byte, error) {
	name := DeviceName(id, fit)
	cache := filepath.Join(Dir(), "device", name)
	if b, err := os.ReadFile(cache); err == nil {
		return b, nil
	}
	raw, err := os.ReadFile(Path(id))
	if err != nil {
		return nil, err
	}
	var out []byte
	if strings.HasSuffix(id, ".gif") {
		out, err = deviceGIF(raw, fit == "contain")
	} else {
		out, err = deviceJPEG(raw, fit == "contain")
	}
	if err != nil {
		return nil, err
	}
	_ = os.MkdirAll(filepath.Dir(cache), 0o755)
	_ = os.WriteFile(cache, out, 0o644)
	return out, nil
}

// target returns the scaled size and the crop for cover/contain.
func target(w, h int, contain bool) (sw, sh int, crop image.Rectangle) {
	if contain {
		if w >= h {
			sw, sh = Screen, max(1, h*Screen/w)
		} else {
			sw, sh = max(1, w*Screen/h), Screen
		}
		return sw, sh, image.Rect(0, 0, sw, sh)
	}
	if w <= h {
		sw, sh = Screen, max(Screen, h*Screen/w)
	} else {
		sw, sh = max(Screen, w*Screen/h), Screen
	}
	x0, y0 := (sw-Screen)/2, (sh-Screen)/2
	return sw, sh, image.Rect(x0, y0, x0+Screen, y0+Screen)
}

func scale(img image.Image, w, h int) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), xdraw.Src, nil)
	return dst
}

// flatten drops alpha onto black (JPEG has none; the device background is dark).
func flatten(img image.Image) image.Image {
	b := img.Bounds()
	dst := image.NewRGBA(b)
	draw.Draw(dst, b, &image.Uniform{color.Black}, image.Point{}, draw.Src)
	draw.Draw(dst, b, img, b.Min, draw.Over)
	return dst
}

func fitted(img image.Image, contain bool) image.Image {
	b := img.Bounds()
	sw, sh, crop := target(b.Dx(), b.Dy(), contain)
	s := scale(img, sw, sh).(*image.RGBA)
	return s.SubImage(crop)
}

func deviceJPEG(raw []byte, contain bool) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	// Go writes baseline JPEG, which the device's TJpgDec decoder needs.
	err = jpeg.Encode(&buf, flatten(fitted(img, contain)), &jpeg.Options{Quality: 88})
	return buf.Bytes(), err
}

// deviceGIF composites every frame (honouring disposal), scales it and writes
// full frames with one shared palette, which the device decoder handles fastest.
func deviceGIF(raw []byte, contain bool) ([]byte, error) {
	g, err := gif.DecodeAll(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	w, h := g.Config.Width, g.Config.Height
	if w == 0 || h == 0 {
		b := g.Image[0].Bounds()
		w, h = b.Dx(), b.Dy()
	}
	pal := color.Palette(palette.Plan9)
	if gp, ok := g.Config.ColorModel.(color.Palette); ok && len(gp) > 0 {
		pal = gp
	}
	canvas := image.NewRGBA(image.Rect(0, 0, w, h))
	out := &gif.GIF{LoopCount: g.LoopCount}
	for i, fr := range g.Image {
		var before *image.RGBA
		if i < len(g.Disposal) && g.Disposal[i] == gif.DisposalPrevious {
			before = image.NewRGBA(canvas.Bounds())
			copy(before.Pix, canvas.Pix)
		}
		draw.Draw(canvas, fr.Bounds(), fr, fr.Bounds().Min, draw.Over)
		f := fitted(canvas, contain)
		pf := image.NewPaletted(image.Rect(0, 0, f.Bounds().Dx(), f.Bounds().Dy()), pal)
		draw.FloydSteinberg.Draw(pf, pf.Bounds(), f, f.Bounds().Min)
		out.Image = append(out.Image, pf)
		delay := 10
		if i < len(g.Delay) && g.Delay[i] > 1 {
			delay = g.Delay[i]
		}
		out.Delay = append(out.Delay, delay)
		out.Disposal = append(out.Disposal, gif.DisposalNone)
		if i < len(g.Disposal) {
			switch g.Disposal[i] {
			case gif.DisposalBackground:
				draw.Draw(canvas, fr.Bounds(), image.Transparent, image.Point{}, draw.Src)
			case gif.DisposalPrevious:
				canvas = before
			}
		}
	}
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, out); err != nil {
		return nil, err
	}
	if buf.Len() > maxDeviceGI {
		return nil, fmt.Errorf("GIF is %.1f MB on the device (max 3 MB): use a shorter one", float64(buf.Len())/1e6)
	}
	return buf.Bytes(), nil
}

// Wanted lists the device files a config needs: name -> (asset id, fit).
func Wanted(c *config.Config) map[string][2]string {
	m := map[string][2]string{}
	for _, p := range c.Profiles {
		for _, pg := range p.Pages {
			if pg.Type != "image" {
				continue
			}
			for _, id := range pg.Images {
				m[DeviceName(id, pg.Fit)] = [2]string{id, pg.Fit}
			}
		}
	}
	return m
}
