// Package icons turns freedesktop icon names (or paths) into small RGB565A8
// bitmaps for the device: mixer rows, window switcher, notifications.
package icons

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	xdraw "golang.org/x/image/draw"
)

const Size = 48

// AvatarSize: Discord avatars are shown up to ~110 px, so they're fetched bigger.
const AvatarSize = 96

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

var (
	themeOnce sync.Once
	themes    []string
)

// themeChain: the icon theme set in GNOME, the themes it inherits, then the fallbacks.
func themeChain() []string {
	themeOnce.Do(func() {
		seen := map[string]bool{}
		var add func(string)
		add = func(t string) {
			if t == "" || seen[t] {
				return
			}
			seen[t] = true
			themes = append(themes, t)
			for _, root := range roots() {
				raw, err := os.ReadFile(filepath.Join(root, t, "index.theme"))
				if err != nil {
					continue
				}
				for _, line := range strings.Split(string(raw), "\n") {
					if v, ok := strings.CutPrefix(strings.TrimSpace(line), "Inherits="); ok {
						for _, p := range strings.Split(v, ",") {
							add(strings.TrimSpace(p))
						}
					}
				}
				break
			}
		}
		if out, err := exec.Command("gsettings", "get", "org.gnome.desktop.interface", "icon-theme").Output(); err == nil {
			add(strings.Trim(strings.TrimSpace(string(out)), "'"))
		}
		for _, t := range []string{"hicolor", "Adwaita", "breeze"} {
			add(t)
		}
	})
	return themes
}

// find returns the best file for an icon name in the user's icon theme first.
func find(name string) string {
	if filepath.IsAbs(name) {
		if _, err := os.Stat(name); err == nil {
			return name
		}
		return ""
	}
	sizes := []string{"64x64", "48x48", "128x128", "96x96", "256x256", "32x32", "512x512"}
	for _, th := range themeChain() {
		for _, root := range roots() {
			for _, sz := range sizes {
				for _, cat := range []string{"apps", "status", "devices"} {
					for _, ext := range []string{".svg", ".png"} { // themes like Papirus keep SVGs in sized dirs
						p := filepath.Join(root, th, sz, cat, name+ext)
						if _, err := os.Stat(p); err == nil {
							return p
						}
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
		// librsvg renders icon themes exactly (opacity, CSS classes); oksvg is the fallback.
		if bin, err := exec.LookPath("rsvg-convert"); err == nil {
			out, err := exec.Command(bin, "-w", fmt.Sprint(Size*2), "-h", fmt.Sprint(Size*2), path).Output()
			if err == nil {
				if img, _, err := image.Decode(bytes.NewReader(out)); err == nil {
					return img
				}
			}
		}
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

// fetchRound downloads an image (Discord avatars) and gives it a round alpha mask.
func fetchRound(u string) image.Image {
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Get(u)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	img, _, err := image.Decode(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil
	}
	dst := image.NewNRGBA(image.Rect(0, 0, AvatarSize, AvatarSize))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), xdraw.Src, nil)
	c := float64(AvatarSize) / 2
	for y := 0; y < AvatarSize; y++ {
		for x := 0; x < AvatarSize; x++ {
			d := math.Hypot(float64(x)+0.5-c, float64(y)+0.5-c)
			cov := math.Max(0, math.Min(1, c-d+0.5))
			o := dst.PixOffset(x, y)
			dst.Pix[o+3] = uint8(float64(dst.Pix[o+3]) * cov)
		}
	}
	return dst
}

// Bitmap returns the icon as RGB565 LE plane + A8 plane, or nil, and its side.
// name is an icon theme name, a file path, or an https URL (made round, AvatarSize).
func Bitmap(name string) []byte {
	b, _ := BitmapSized(name)
	return b
}

func BitmapSized(name string) ([]byte, int) {
	if name == "" {
		return nil, 0
	}
	size := Size
	if strings.HasPrefix(name, "https://") {
		size = AvatarSize
	}
	mu.Lock()
	if b, ok := cache[name]; ok {
		mu.Unlock()
		return b, size
	}
	mu.Unlock()
	var src image.Image
	if size == AvatarSize {
		src = fetchRound(name)
	} else if p := find(name); p != "" {
		src = load(p)
	}
	var out []byte
	if src != nil {
		dst := image.NewNRGBA(image.Rect(0, 0, size, size))
		if b := src.Bounds(); b.Dx() == size && b.Dy() == size {
			draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Src)
		} else {
			xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, b, xdraw.Src, nil)
		}
		out = make([]byte, size*size*3)
		for i := 0; i < size*size; i++ {
			p := dst.Pix[i*4:]
			v := uint16(p[0]>>3)<<11 | uint16(p[1]>>2)<<5 | uint16(p[2]>>3)
			binary.LittleEndian.PutUint16(out[2*i:], v)
			out[2*size*size+i] = p[3]
		}
	}
	mu.Lock()
	cache[name] = out
	mu.Unlock()
	return out, size
}
