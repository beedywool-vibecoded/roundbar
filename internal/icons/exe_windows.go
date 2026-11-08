package icons

import (
	"image"
	"unsafe"

	"github.com/beedywool-vibecoded/roundbar/internal/win"
	"golang.org/x/sys/windows"
)

var (
	procPrivateExtractIconsW = win.User32("PrivateExtractIconsW")
	procGetIconInfo          = win.User32("GetIconInfo")
	procDestroyIcon          = win.User32("DestroyIcon")
	procGetDC                = win.User32("GetDC")
	procReleaseDC            = win.User32("ReleaseDC")
	gdi32                    = windows.NewLazySystemDLL("gdi32.dll")
	procGetDIBits            = gdi32.NewProc("GetDIBits")
	procDeleteObject         = gdi32.NewProc("DeleteObject")
)

type bitmapInfoHeader struct {
	size                  uint32
	width, height         int32
	planes, bitCount      uint16
	compression, sizeImg  uint32
	xPels, yPels          int32
	clrUsed, clrImportant uint32
}

// exeIcon extracts the executable's main icon at size×size as an image.
func exeIcon(path string, size int) image.Image {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil
	}
	var hicon uintptr
	if n, _, _ := procPrivateExtractIconsW.Call(uintptr(unsafe.Pointer(p)), 0, uintptr(size), uintptr(size),
		uintptr(unsafe.Pointer(&hicon)), 0, 1, 0); n == 0 || n == 0xFFFFFFFF || hicon == 0 {
		return nil
	}
	defer procDestroyIcon.Call(hicon)
	var info struct {
		isIcon             int32
		xHotspot, yHotspot uint32
		hbmMask, hbmColor  uintptr
	}
	if r, _, _ := procGetIconInfo.Call(hicon, uintptr(unsafe.Pointer(&info))); r == 0 {
		return nil
	}
	defer procDeleteObject.Call(info.hbmMask)
	if info.hbmColor == 0 {
		return nil // monochrome icon
	}
	defer procDeleteObject.Call(info.hbmColor)
	dc, _, _ := procGetDC.Call(0)
	defer procReleaseDC.Call(0, dc)
	bmi := bitmapInfoHeader{width: int32(size), height: -int32(size), planes: 1, bitCount: 32} // top-down BGRA
	bmi.size = uint32(unsafe.Sizeof(bmi))
	bgra := make([]byte, size*size*4)
	if r, _, _ := procGetDIBits.Call(dc, info.hbmColor, 0, uintptr(size), uintptr(unsafe.Pointer(&bgra[0])), uintptr(unsafe.Pointer(&bmi)), 0); r == 0 {
		return nil
	}
	hasAlpha := false
	for i := 3; i < len(bgra); i += 4 {
		if bgra[i] != 0 {
			hasAlpha = true
			break
		}
	}
	var mask []byte
	if !hasAlpha { // old icons: transparency is in the AND mask
		mask = make([]byte, size*size*4)
		mi := bmi
		if r, _, _ := procGetDIBits.Call(dc, info.hbmMask, 0, uintptr(size), uintptr(unsafe.Pointer(&mask[0])), uintptr(unsafe.Pointer(&mi)), 0); r == 0 {
			mask = nil
		}
	}
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for i := 0; i < size*size; i++ {
		b, g, r, a := bgra[4*i], bgra[4*i+1], bgra[4*i+2], bgra[4*i+3]
		if !hasAlpha {
			a = 255
			if mask != nil && mask[4*i] != 0 {
				a = 0
			}
		} else if a > 0 && a < 255 { // premultiplied
			r, g, b = uint8(int(r)*255/int(a)), uint8(int(g)*255/int(a)), uint8(int(b)*255/int(a))
		}
		img.Pix[4*i], img.Pix[4*i+1], img.Pix[4*i+2], img.Pix[4*i+3] = r, g, b, a
	}
	return img
}
