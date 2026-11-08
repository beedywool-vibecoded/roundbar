// fontpack pre-renders the CJK and emoji glyphs the device can't get from its
// Latin font into one binary blob that lives in the "fonts" flash partition.
//
//	go run ./tools/fontpack -out firmware/build/fonts.bin
//
// Text comes from the system Noto Sans CJK (JP forms for kana and JIS kanji,
// SC for GB2312-only hanzi, TC for Big5-only ones, KR for Hangul); emoji from
// Noto Color Emoji's CBDT bitmaps. Layout is documented in firmware/main/fontpack.c.
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"log"
	"math"
	"os"
	"sort"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
)

// Pixel sizes match the device's Google Sans Flex sizes (title, body, label).
type sizeSpec struct {
	px    int
	bold  bool
	emoji bool
}

var sizes = []sizeSpec{{22, true, true}, {16, false, true}, {14, false, false}}

const emScale = 0.86 // TinyTTF sizes are ascent+descent; CJK glyphs fill ~1 em

func main() {
	out := flag.String("out", "fonts.bin", "output file")
	cjkDir := flag.String("cjk", "/usr/share/fonts/noto-cjk", "directory with NotoSansCJK-{Medium,Bold}.ttc")
	emojiPath := flag.String("emoji", "/usr/share/fonts/noto/NotoColorEmoji.ttf", "Noto Color Emoji")
	flag.Parse()

	sets := charsets()
	log.Printf("glyphs: %d JP, %d SC-only, %d TC-only, %d KR", len(sets["jp"]), len(sets["sc"]), len(sets["tc"]), len(sets["kr"]))

	var fonts [][]byte
	for _, sz := range sizes {
		weight := "Medium"
		if sz.bold {
			weight = "Bold"
		}
		faces := loadCJK(*cjkDir+"/NotoSansCJK-"+weight+".ttc", float64(sz.px)*emScale)
		fonts = append(fonts, buildText(sz.px, faces, sets))
	}
	emoji := loadEmoji(*emojiPath)
	log.Printf("emoji: %d single-codepoint bitmaps", len(emoji))
	for _, sz := range sizes {
		if sz.emoji {
			fonts = append(fonts, buildEmoji(sz.px, emoji))
		}
	}

	var blob bytes.Buffer
	blob.WriteString("RBF1")
	binary.Write(&blob, binary.LittleEndian, uint32(len(fonts)))
	off := uint32(8 + 4*len(fonts))
	for _, f := range fonts {
		binary.Write(&blob, binary.LittleEndian, off)
		off += uint32(len(f))
	}
	for _, f := range fonts {
		blob.Write(f)
	}
	if err := os.WriteFile(*out, blob.Bytes(), 0o644); err != nil {
		log.Fatal(err)
	}
	log.Printf("wrote %s: %.1f MB", *out, float64(blob.Len())/1e6)
}

// ---- character sets ----------------------------------------------------------------------

func decodeAll(enc encoding.Encoding, ranges [][4]byte) map[rune]bool {
	m := map[rune]bool{}
	dec := enc.NewDecoder()
	for _, r := range ranges {
		for hi := int(r[0]); hi <= int(r[1]); hi++ {
			for lo := int(r[2]); lo <= int(r[3]); lo++ {
				s, err := dec.Bytes([]byte{byte(hi), byte(lo)})
				if err != nil {
					continue
				}
				for _, c := range string(s) {
					if c != '�' && c > 0x2000 {
						m[c] = true
					}
				}
			}
		}
	}
	return m
}

func charsets() map[string][]rune {
	jp := decodeAll(japanese.EUCJP, [][4]byte{{0xA1, 0xF4, 0xA1, 0xFE}}) // JIS X 0208: symbols, kana, levels 1+2
	for c := rune(0x3000); c <= 0x30FF; c++ {                            // CJK punctuation, kana
		jp[c] = true
	}
	for c := rune(0xFF01); c <= 0xFF9F; c++ { // full/halfwidth forms
		jp[c] = true
	}
	gb := decodeAll(simplifiedchinese.GBK, [][4]byte{{0xA1, 0xF7, 0xA1, 0xFE}}) // GB2312 area of GBK
	// Big5 level 1 (frequent characters) only; level 2 doubles the size for rare ones
	big5 := decodeAll(traditionalchinese.Big5, [][4]byte{{0xA4, 0xC6, 0x40, 0x7E}, {0xA4, 0xC6, 0xA1, 0xFE}})
	kr := decodeAll(korean.EUCKR, [][4]byte{{0xB0, 0xC8, 0xA1, 0xFE}}) // KS X 1001 Hangul syllables
	for c := rune(0x3131); c <= 0x318E; c++ {                          // Hangul jamo
		kr[c] = true
	}
	sc, tc := map[rune]bool{}, map[rune]bool{}
	for c := range gb {
		if !jp[c] {
			sc[c] = true
		}
	}
	for c := range big5 {
		if !jp[c] && !sc[c] {
			tc[c] = true
		}
	}
	for c := range kr {
		if jp[c] || sc[c] || tc[c] {
			delete(kr, c)
		}
	}
	list := func(m map[rune]bool) []rune {
		r := make([]rune, 0, len(m))
		for c := range m {
			r = append(r, c)
		}
		return r
	}
	return map[string][]rune{"jp": list(jp), "sc": list(sc), "tc": list(tc), "kr": list(kr)}
}

// ---- text glyphs -------------------------------------------------------------------------

func loadCJK(path string, em float64) map[string]font.Face {
	raw, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("%v (install noto-fonts-cjk)", err)
	}
	coll, err := opentype.ParseCollection(raw)
	if err != nil {
		log.Fatal(err)
	}
	faces := map[string]font.Face{}
	var buf sfnt.Buffer
	for i := 0; i < coll.NumFonts(); i++ {
		f, _ := coll.Font(i)
		name, _ := f.Name(&buf, sfnt.NameIDFamily)
		for _, tag := range []string{"JP", "SC", "TC", "KR"} {
			if name == "Noto Sans CJK "+tag || strings.HasPrefix(name, "Noto Sans CJK "+tag+" ") {
				if _, dup := faces[strings.ToLower(tag)]; dup {
					continue
				}
				face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: em, DPI: 72, Hinting: font.HintingNone})
				if err != nil {
					log.Fatal(err)
				}
				faces[strings.ToLower(tag)] = face
			}
		}
	}
	if len(faces) < 4 {
		log.Fatalf("%s: found only %d of JP/SC/TC/KR", path, len(faces))
	}
	return faces
}

type glyph struct {
	cp               rune
	adv              int
	w, h, ofsX, ofsY int
	data             []byte // A4 rows, or RGB565 then A8 for color glyphs
}

func buildText(px int, faces map[string]font.Face, sets map[string][]rune) []byte {
	var gs []glyph
	for tag, runes := range sets {
		face := faces[tag]
		for _, c := range runes {
			dr, mask, mp, adv, ok := face.Glyph(fixed.Point26_6{}, c)
			if !ok {
				continue
			}
			w, h := dr.Dx(), dr.Dy()
			g := glyph{cp: c, adv: adv.Round(), w: w, h: h, ofsX: dr.Min.X, ofsY: -dr.Max.Y}
			stride := (w + 1) / 2
			g.data = make([]byte, stride*h)
			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					_, _, _, a := mask.At(mp.X+x, mp.Y+y).RGBA()
					v := byte((a*15 + 0x7FFF) / 0xFFFF)
					if x%2 == 0 {
						g.data[y*stride+x/2] |= v << 4
					} else {
						g.data[y*stride+x/2] |= v
					}
				}
			}
			gs = append(gs, g)
		}
	}
	m := faces["jp"].Metrics()
	return encodeFont(px, m.Height.Round(), m.Descent.Round(), false, gs)
}

// ---- emoji (CBDT) -----------------------------------------------------------------------

type tables map[string][]byte

func readTables(raw []byte) tables {
	t := tables{}
	n := int(binary.BigEndian.Uint16(raw[4:]))
	for i := 0; i < n; i++ {
		rec := raw[12+16*i:]
		tag := string(rec[:4])
		off, ln := binary.BigEndian.Uint32(rec[8:]), binary.BigEndian.Uint32(rec[12:])
		t[tag] = raw[off : off+ln]
	}
	return t
}

// cmap format 12 -> glyph id
func cmap12(cm []byte) map[rune]uint16 {
	m := map[rune]uint16{}
	n := int(binary.BigEndian.Uint16(cm[2:]))
	for i := 0; i < n; i++ {
		rec := cm[4+8*i:]
		off := binary.BigEndian.Uint32(rec[4:])
		sub := cm[off:]
		if binary.BigEndian.Uint16(sub) != 12 {
			continue
		}
		groups := int(binary.BigEndian.Uint32(sub[12:]))
		for g := 0; g < groups; g++ {
			gr := sub[16+12*g:]
			start, end, gid := binary.BigEndian.Uint32(gr), binary.BigEndian.Uint32(gr[4:]), binary.BigEndian.Uint32(gr[8:])
			for c := start; c <= end; c++ {
				m[rune(c)] = uint16(gid + c - start)
			}
		}
		return m
	}
	return m
}

func loadEmoji(path string) map[rune]image.Image {
	raw, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("%v (install noto-fonts-emoji)", err)
	}
	t := readTables(raw)
	cblc, cbdt := t["CBLC"], t["CBDT"]
	if cblc == nil || cbdt == nil {
		log.Fatal("emoji font has no CBLC/CBDT tables")
	}
	gidPNG := map[uint16][]byte{}
	numSizes := int(binary.BigEndian.Uint32(cblc[4:]))
	for s := 0; s < numSizes && s < 1; s++ { // the first strike is the 109 px one
		bs := cblc[8+48*s:]
		arrOff := binary.BigEndian.Uint32(bs)
		nSub := int(binary.BigEndian.Uint32(bs[8:]))
		for i := 0; i < nSub; i++ {
			e := cblc[int(arrOff)+8*i:]
			first, last := binary.BigEndian.Uint16(e), binary.BigEndian.Uint16(e[2:])
			sub := cblc[arrOff+binary.BigEndian.Uint32(e[4:]):]
			idxFmt, imgFmt := binary.BigEndian.Uint16(sub), binary.BigEndian.Uint16(sub[2:])
			dataOff := binary.BigEndian.Uint32(sub[4:])
			offs := func(k int) uint32 {
				switch idxFmt {
				case 1:
					return binary.BigEndian.Uint32(sub[8+4*k:])
				case 3:
					return uint32(binary.BigEndian.Uint16(sub[8+2*k:]))
				}
				return 0
			}
			if idxFmt != 1 && idxFmt != 3 {
				continue
			}
			for g := int(first); g <= int(last); g++ {
				k := g - int(first)
				start, end := dataOff+offs(k), dataOff+offs(k+1)
				if end <= start {
					continue
				}
				d := cbdt[start:end]
				switch imgFmt {
				case 17: // smallGlyphMetrics(5) + len + png
					gidPNG[uint16(g)] = d[9 : 9+binary.BigEndian.Uint32(d[5:])]
				case 18: // bigGlyphMetrics(8) + len + png
					gidPNG[uint16(g)] = d[12 : 12+binary.BigEndian.Uint32(d[8:])]
				case 19:
					gidPNG[uint16(g)] = d[4 : 4+binary.BigEndian.Uint32(d)]
				}
			}
		}
	}
	out := map[rune]image.Image{}
	for c, gid := range cmap12(t["cmap"]) {
		if c < 0x80 || (c >= 0xFE00 && c <= 0xFE0F) || (c >= 0x1F3FB && c <= 0x1F3FF) || (c >= 0xE0000) {
			continue // ASCII digits/#/* keycap bases, variation selectors, skin tones, tags
		}
		p, ok := gidPNG[gid]
		if !ok {
			continue
		}
		img, err := png.Decode(bytes.NewReader(p))
		if err != nil {
			continue
		}
		out[c] = img
	}
	return out
}

// downscale with area averaging in premultiplied space
func downscale(src image.Image, w, h int) *image.NRGBA {
	b := src.Bounds()
	rgba := image.NewRGBA(b)
	draw.Draw(rgba, b, src, b.Min, draw.Src)
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	sx, sy := float64(b.Dx())/float64(w), float64(b.Dy())/float64(h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var r, g, bl, a, n float64
			for yy := int(float64(y) * sy); yy < int(math.Ceil(float64(y+1)*sy)) && yy < b.Dy(); yy++ {
				for xx := int(float64(x) * sx); xx < int(math.Ceil(float64(x+1)*sx)) && xx < b.Dx(); xx++ {
					o := rgba.PixOffset(b.Min.X+xx, b.Min.Y+yy)
					r += float64(rgba.Pix[o])
					g += float64(rgba.Pix[o+1])
					bl += float64(rgba.Pix[o+2])
					a += float64(rgba.Pix[o+3])
					n++
				}
			}
			o := dst.PixOffset(x, y)
			if a > 0 {
				dst.Pix[o], dst.Pix[o+1], dst.Pix[o+2] = byte(r*255/a), byte(g*255/a), byte(bl*255/a)
			}
			dst.Pix[o+3] = byte(a / n)
		}
	}
	return dst
}

func buildEmoji(px int, emoji map[rune]image.Image) []byte {
	var gs []glyph
	size := int(math.Round(float64(px) * 1.05))
	for c, img := range emoji {
		b := img.Bounds()
		w := int(math.Round(float64(size) * float64(b.Dx()) / float64(b.Dy())))
		im := downscale(img, w, size)
		g := glyph{cp: c, adv: w + 2, w: w, h: size, ofsX: 1, ofsY: -int(math.Round(float64(px) * 0.18))}
		g.data = make([]byte, w*size*3) // RGB565 LE plane, then A8 plane (LV_COLOR_FORMAT_RGB565A8)
		for i := 0; i < w*size; i++ {
			p := im.Pix[i*4:]
			v := uint16(p[0]>>3)<<11 | uint16(p[1]>>2)<<5 | uint16(p[2]>>3)
			binary.LittleEndian.PutUint16(g.data[2*i:], v)
			g.data[2*w*size+i] = p[3]
		}
		gs = append(gs, g)
	}
	return encodeFont(px, size, int(math.Round(float64(px)*0.18)), true, gs)
}

// ---- encoding ----------------------------------------------------------------------------

// font: u16 px, u16 line_height, u16 base_line, u16 flags(bit0 color), u32 count,
// u32 cmap_off, u32 dsc_off, u32 bmp_off (offsets from font start),
// cmap: u32 codepoints (sorted), dsc: {u32 bmp_off, u16 adv, u8 w, u8 h, i8 ofs_x, i8 ofs_y, u16 pad}.
func encodeFont(px, lineHeight, baseLine int, color bool, gs []glyph) []byte {
	kept := gs[:0]
	for _, g := range gs {
		if g.w <= 255 && g.h <= 255 {
			kept = append(kept, g)
		}
	}
	gs = kept
	sort.Slice(gs, func(i, j int) bool { return gs[i].cp < gs[j].cp })
	const hdr = 24
	n := len(gs)
	cmapOff, dscOff := hdr, hdr+4*n
	bmpOff := dscOff + 12*n
	var cm, ds, bm bytes.Buffer
	for _, g := range gs {
		binary.Write(&cm, binary.LittleEndian, uint32(g.cp))
		binary.Write(&ds, binary.LittleEndian, uint32(bm.Len()))
		binary.Write(&ds, binary.LittleEndian, uint16(g.adv))
		ds.Write([]byte{byte(g.w), byte(g.h), byte(int8(g.ofsX)), byte(int8(g.ofsY)), 0, 0})
		bm.Write(g.data)
		for bm.Len()%4 != 0 { // keep RGB565 planes aligned
			bm.WriteByte(0)
		}
	}
	var out bytes.Buffer
	flags := uint16(0)
	if color {
		flags = 1
	}
	binary.Write(&out, binary.LittleEndian, []uint16{uint16(px), uint16(lineHeight), uint16(baseLine), flags})
	binary.Write(&out, binary.LittleEndian, []uint32{uint32(cm.Len() / 4), uint32(cmapOff), uint32(dscOff), uint32(bmpOff)})
	out.Write(cm.Bytes())
	out.Write(ds.Bytes())
	out.Write(bm.Bytes())
	for out.Len()%4 != 0 {
		out.WriteByte(0)
	}
	fmt.Fprintf(os.Stderr, "  %2dpx %s: %d glyphs, %.2f MB\n", px, map[bool]string{false: "text ", true: "emoji"}[color], cm.Len()/4, float64(out.Len())/1e6)
	return out.Bytes()
}
