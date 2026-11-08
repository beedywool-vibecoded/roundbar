// Package cover turns an MPRIS album-art URL into what the device shows behind
// media pages: a blurred 360×360 RGB565 image and an M3-style tonal palette.
// It only runs on track change, so plain Go image code is cheap enough.
package cover

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"time"

	_ "golang.org/x/image/webp"
)

const Size = 360

// Load fetches and decodes the artwork (file:// or http(s)://).
func Load(ctx context.Context, artURL string) (image.Image, error) {
	u, err := url.Parse(artURL)
	if err != nil {
		return nil, err
	}
	var r io.ReadCloser
	switch u.Scheme {
	case "file":
		f, err := os.Open(u.Path)
		if err != nil {
			return nil, err
		}
		r = f
	case "http", "https":
		ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, artURL, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("artwork: HTTP %d", resp.StatusCode)
		}
		r = resp.Body
	default:
		return nil, errors.New("artwork: unsupported URL scheme " + u.Scheme)
	}
	defer r.Close()
	img, _, err := image.Decode(io.LimitReader(r, 16<<20))
	return img, err
}

// area-average downscale of the centered square crop to n×n, linear RGB floats
func shrink(img image.Image, n int) [][3]float64 {
	b := img.Bounds()
	side := min(b.Dx(), b.Dy())
	x0, y0 := b.Min.X+(b.Dx()-side)/2, b.Min.Y+(b.Dy()-side)/2
	out := make([][3]float64, n*n)
	cnt := make([]float64, n*n)
	step := max(1, side/(n*8)) // sample at most ~8×8 source pixels per cell
	for y := 0; y < side; y += step {
		cy := y * n / side
		for x := 0; x < side; x += step {
			cx := x * n / side
			r, g, bl, _ := img.At(x0+x, y0+y).RGBA()
			i := cy*n + cx
			out[i][0] += float64(r) / 65535
			out[i][1] += float64(g) / 65535
			out[i][2] += float64(bl) / 65535
			cnt[i]++
		}
	}
	for i := range out {
		if cnt[i] > 0 {
			out[i][0] /= cnt[i]
			out[i][1] /= cnt[i]
			out[i][2] /= cnt[i]
		}
	}
	return out
}

// Blurred renders the background: shrink to 32×32, blur that once, then bilinear back to 360×360
// (a cheap, very smooth blur), as little-endian RGB565 with ordered dithering.
func Blurred(img image.Image) []byte {
	const n = 32
	small := shrink(img, n)
	for pass := 0; pass < 1; pass++ { // 3×3 box blur on the small grid: Gaussian-like
		next := make([][3]float64, n*n)
		for y := 0; y < n; y++ {
			for x := 0; x < n; x++ {
				var acc [3]float64
				for dy := -1; dy <= 1; dy++ {
					for dx := -1; dx <= 1; dx++ {
						c := small[min(max(y+dy, 0), n-1)*n+min(max(x+dx, 0), n-1)]
						acc[0], acc[1], acc[2] = acc[0]+c[0], acc[1]+c[1], acc[2]+c[2]
					}
				}
				next[y*n+x] = [3]float64{acc[0] / 9, acc[1] / 9, acc[2] / 9}
			}
		}
		small = next
	}
	out := make([]byte, Size*Size*2)
	bayer := [4][4]float64{{0, 8, 2, 10}, {12, 4, 14, 6}, {3, 11, 1, 9}, {15, 7, 13, 5}}
	for y := 0; y < Size; y++ {
		fy := (float64(y)+0.5)*n/Size - 0.5
		y0 := int(math.Floor(fy))
		ty := fy - float64(y0)
		for x := 0; x < Size; x++ {
			fx := (float64(x)+0.5)*n/Size - 0.5
			x0 := int(math.Floor(fx))
			tx := fx - float64(x0)
			at := func(xx, yy int) [3]float64 {
				return small[min(max(yy, 0), n-1)*n+min(max(xx, 0), n-1)]
			}
			var c [3]float64
			for k := 0; k < 3; k++ {
				top := at(x0, y0)[k]*(1-tx) + at(x0+1, y0)[k]*tx
				bot := at(x0, y0+1)[k]*(1-tx) + at(x0+1, y0+1)[k]*tx
				c[k] = top*(1-ty) + bot*ty
			}
			d := (bayer[y&3][x&3]/16 - 0.5)
			r := clamp8(c[0]*255 + d*8)
			g := clamp8(c[1]*255 + d*4)
			b := clamp8(c[2]*255 + d*8)
			v := uint16(r>>3)<<11 | uint16(g>>2)<<5 | uint16(b>>3)
			binary.LittleEndian.PutUint16(out[2*(y*Size+x):], v)
		}
	}
	return out
}

func clamp8(v float64) uint8 {
	return uint8(math.Max(0, math.Min(255, math.Round(v))))
}

// ---- palette ---------------------------------------------------------------------------

type lab struct{ L, a, b float64 }

func lin(c float64) float64 {
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

func gam(c float64) float64 {
	if c <= 0.0031308 {
		return 12.92 * c
	}
	return 1.055*math.Pow(c, 1/2.4) - 0.055
}

func toLab(r, g, b float64) lab {
	r, g, b = lin(r), lin(g), lin(b)
	x := (0.4124*r + 0.3576*g + 0.1805*b) / 0.95047
	y := 0.2126*r + 0.7152*g + 0.0722*b
	z := (0.0193*r + 0.1192*g + 0.9505*b) / 1.08883
	f := func(t float64) float64 {
		if t > 0.008856 {
			return math.Cbrt(t)
		}
		return 7.787*t + 16.0/116
	}
	fx, fy, fz := f(x), f(y), f(z)
	return lab{116*fy - 16, 500 * (fx - fy), 200 * (fy - fz)}
}

// fromLCh returns sRGB 0..1 and whether it was in gamut.
func fromLCh(L, C, h float64) (r, g, b float64, ok bool) {
	a, bb := C*math.Cos(h), C*math.Sin(h)
	fy := (L + 16) / 116
	fx, fz := fy+a/500, fy-bb/200
	inv := func(t float64) float64 {
		if t*t*t > 0.008856 {
			return t * t * t
		}
		return (t - 16.0/116) / 7.787
	}
	x, y, z := inv(fx)*0.95047, inv(fy), inv(fz)*1.08883
	r = gam(3.2406*x - 1.5372*y - 0.4986*z)
	g = gam(-0.9689*x + 1.8758*y + 0.0415*z)
	b = gam(0.0557*x - 0.2040*y + 1.0570*z)
	ok = r >= -0.001 && r <= 1.001 && g >= -0.001 && g <= 1.001 && b >= -0.001 && b <= 1.001
	return
}

// tone gives the color at lightness L (the M3 "tone"), hue h, reducing chroma
// until it fits in sRGB.
func tone(L, C, h float64) string {
	for c := C; c >= 0; c -= 1 {
		if r, g, b, ok := fromLCh(L, c, h); ok || c == 0 {
			return fmt.Sprintf("#%02X%02X%02X", clamp8(r*255), clamp8(g*255), clamp8(b*255))
		}
	}
	return "#808080"
}

// seed picks the cover's characteristic hue: the most populated hue bucket,
// weighted by chroma so colorful areas beat large gray ones.
func seed(img image.Image) (hue, chroma float64) {
	cells := shrink(img, 48)
	var bucket [36]float64
	var sumA, sumB [36]float64
	for _, c := range cells {
		l := toLab(c[0], c[1], c[2])
		ch := math.Hypot(l.a, l.b)
		if ch < 8 || l.L < 12 || l.L > 95 {
			continue
		}
		h := math.Atan2(l.b, l.a)
		i := int((h+math.Pi)/(2*math.Pi)*36) % 36
		w := ch * ch
		bucket[i] += w
		sumA[i] += l.a * w
		sumB[i] += l.b * w
	}
	best, bestW := -1, 0.0
	for i := range bucket {
		w := bucket[i] + 0.5*(bucket[(i+35)%36]+bucket[(i+1)%36])
		if w > bestW {
			best, bestW = i, w
		}
	}
	if best < 0 {
		return 0, 0 // grayscale cover
	}
	h := math.Atan2(sumB[best], sumA[best])
	return h, math.Sqrt(bucket[best] / float64(len(cells)))
}

// Palette builds the theme roles (docs/spec.md §1) from the cover, tonal-spot
// style: primary at the seed hue, tertiary rotated 60°, near-neutral surfaces.
func Palette(img image.Image, dark bool, errorRoles map[string]string) map[string]string {
	h, c := seed(img)
	pc := math.Min(math.Max(c, 0)+24, 48)
	if c == 0 {
		pc = 4
	}
	sc, tc, nc, nvc := math.Min(pc, 16), math.Min(pc, 30), math.Min(pc, 5), math.Min(pc, 9)
	th := h + math.Pi/3
	p := func(L float64) string { return tone(L, pc, h) }
	s := func(L float64) string { return tone(L, sc, h) }
	t := func(L float64) string { return tone(L, tc, th) }
	n := func(L float64) string { return tone(L, nc, h) }
	nv := func(L float64) string { return tone(L, nvc, h) }
	var m map[string]string
	if dark {
		m = map[string]string{
			"surface": n(6), "surfaceContainer": n(12), "surfaceContainerHigh": n(17), "surfaceContainerHighest": n(22),
			"onSurface": n(90), "onSurfaceVariant": nv(80), "primary": p(80), "onPrimary": p(20),
			"primaryContainer": p(30), "onPrimaryContainer": p(90), "secondaryContainer": s(30), "onSecondaryContainer": s(90),
			"tertiary": t(80), "tertiaryContainer": t(30), "onTertiaryContainer": t(90), "outline": nv(60), "outlineVariant": nv(30),
		}
	} else {
		m = map[string]string{
			"surface": n(98), "surfaceContainer": n(94), "surfaceContainerHigh": n(92), "surfaceContainerHighest": n(90),
			"onSurface": n(10), "onSurfaceVariant": nv(30), "primary": p(40), "onPrimary": p(100),
			"primaryContainer": p(90), "onPrimaryContainer": p(10), "secondaryContainer": s(90), "onSecondaryContainer": s(10),
			"tertiary": t(40), "tertiaryContainer": t(90), "onTertiaryContainer": t(10), "outline": nv(50), "outlineVariant": nv(80),
		}
	}
	for _, k := range []string{"error", "errorContainer", "onErrorContainer"} {
		m[k] = errorRoles[k]
	}
	return m
}

// Chunks splits raw bytes for the serial protocol.
func Chunks(raw []byte, size int) [][]byte {
	var out [][]byte
	for len(raw) > 0 {
		n := min(size, len(raw))
		out = append(out, raw[:n])
		raw = raw[n:]
	}
	return out
}
