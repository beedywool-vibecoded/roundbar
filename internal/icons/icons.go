// Package icons turns freedesktop icon names (or paths) into small RGB565A8
// bitmaps for the device: mixer rows, window switcher, notifications.
package icons

import (
	"encoding/binary"
	"image"
	"image/draw"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	xdraw "golang.org/x/image/draw"
)

const Size = 40

var (
	mu    sync.Mutex
	cache = map[string][]byte{}
)

func roots() []string {
	home, _ := os.UserHomeDir()
	r := []string{
		filepath.Join(home, ".local/share/icons"),
		filepath.Join(home, ".local/share/flatpak/exports/share/icons"),
		"/var/lib/flatpak/exports/share/icons",
		"/usr/share/icons",
	}
	return r
}

// find returns the best file for an icon name: big PNGs first, then SVG.
func find(name string) string {
	if filepath.IsAbs(name) {
		if _, err := os.Stat(name); err == nil {
			return name
		}
		return ""
	}
	themes := []string{"hicolor", "Adwaita", "breeze"}
	sizes := []string{"64x64", "48x48", "128x128", "96x96", "256x256", "32x32", "512x512"}
	for _, root := range roots() {
		for _, th := range themes {
			for _, sz := range sizes {
				for _, cat := range []string{"apps", "status", "devices"} {
					p := filepath.Join(root, th, sz, cat, name+".png")
					if _, err := os.Stat(p); err == nil {
						return p
					}
				}
			}
			for _, cat := range []string{"apps", "status", "devices"} {
				for _, sub := range []string{"scalable", "symbolic"} {
					suffix := ".svg"
					if sub == "symbolic" {
						suffix = "-symbolic.svg"
					}
					p := filepath.Join(root, th, sub, cat, name+suffix)
					if _, err := os.Stat(p); err == nil {
						return p
					}
				}
			}
		}
	}
	for _, ext := range []string{".png", ".svg"} {
		p := filepath.Join("/usr/share/pixmaps", name+ext)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func load(path string) image.Image {
	if strings.HasSuffix(path, ".svg") {
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()
		icon, err := oksvg.ReadIconStream(f, oksvg.IgnoreErrorMode)
		if err != nil {
			return nil
		}
		icon.SetTarget(0, 0, Size, Size)
		rgba := image.NewRGBA(image.Rect(0, 0, Size, Size))
		icon.Draw(rasterx.NewDasher(Size, Size, rasterx.NewScannerGV(Size, Size, rgba, rgba.Bounds())), 1)
		return rgba
	}
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil
	}
	return img
}

// Bitmap returns the icon as RGB565 LE plane + A8 plane (Size×Size), or nil.
func Bitmap(name string) []byte {
	if name == "" {
		return nil
	}
	mu.Lock()
	if b, ok := cache[name]; ok {
		mu.Unlock()
		return b
	}
	mu.Unlock()
	var out []byte
	if p := find(name); p != "" {
		if img := load(p); img != nil {
			dst := image.NewNRGBA(image.Rect(0, 0, Size, Size))
			if b := img.Bounds(); b.Dx() == Size && b.Dy() == Size {
				draw.Draw(dst, dst.Bounds(), img, b.Min, draw.Src)
			} else {
				xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, b, xdraw.Src, nil)
			}
			out = make([]byte, Size*Size*3)
			for i := 0; i < Size*Size; i++ {
				p := dst.Pix[i*4:]
				v := uint16(p[0]>>3)<<11 | uint16(p[1]>>2)<<5 | uint16(p[2]>>3)
				binary.LittleEndian.PutUint16(out[2*i:], v)
				out[2*Size*Size+i] = p[3]
			}
		}
	}
	mu.Lock()
	cache[name] = out
	mu.Unlock()
	return out
}
