package codec

import (
	"image"
	"image/color"
	"slices"

	"github.com/superiority-dev/thumbhash/internal/ycbcr"
)

// reciprocals holds 1/a for each alpha byte a.
var reciprocals = func() (t [256]float64) {
	for a := 1; a < len(t); a++ {
		t[a] = 1 / float64(a)
	}

	return t
}()

// Straight appends img's pixels to dst in row order, each as the 8-bit
// straight RGBA that [color.NRGBAModel] converts it to. It reads NRGBA, RGBA
// and YCbCr images from their pixel slices, and any other image through At.
func Straight(dst []byte, img image.Image) []byte {
	b := img.Bounds()
	if b.Empty() {
		return dst
	}

	switch img := img.(type) {
	case *image.NRGBA:
		for y := b.Min.Y; y < b.Max.Y; y++ {
			i := img.PixOffset(b.Min.X, y)
			dst = append(dst, img.Pix[i:i+4*b.Dx()]...)
		}

		return dst
	case *image.RGBA:
		start := len(dst)
		dst = slices.Grow(dst, 4*b.Dx()*b.Dy())[:start+4*b.Dx()*b.Dy()]
		out := dst[start:]

		for y := b.Min.Y; y < b.Max.Y; y++ {
			row := img.Pix[img.PixOffset(b.Min.X, y):][:4*b.Dx()]
			outRow := out[4*(y-b.Min.Y)*b.Dx():][:len(row)]

			if allOpaque(row) {
				copy(outRow, row)

				continue
			}

			for i := 0; i < len(row); i += 4 {
				unpremultiply((*[4]byte)(outRow[i:i+4]), (*[4]byte)(row[i:i+4]))
			}
		}

		return dst
	case *image.YCbCr:
		start := len(dst)
		dst = slices.Grow(dst, 4*b.Dx()*b.Dy())[:start+4*b.Dx()*b.Dy()]
		out := dst[start:]
		x0 := b.Min.X

		for y := b.Min.Y; y < b.Max.Y; y++ {
			ys := img.Y[img.YOffset(x0, y):]
			_ = ys[b.Dx()-1] // At checks the plane's length; slicing checks only its capacity
			ys = ys[:b.Dx()]
			c := img.COffset(x0, y)
			cb, cr := img.Cb[c:], img.Cr[c:]
			outRow := out[4*(y-b.Min.Y)*b.Dx():][:4*len(ys)]

			// Chroma indexes as COffset computes them, truncating division included.
			switch img.SubsampleRatio {
			case image.YCbCrSubsampleRatio422, image.YCbCrSubsampleRatio420:
				for x, v := range ys {
					i := (x0+x)/2 - x0/2
					r, g, bl := ycbcr.RGB(v, cb[i], cr[i])
					*(*[4]byte)(outRow[4*x:]) = [4]byte{r, g, bl, 0xff}
				}
			case image.YCbCrSubsampleRatio411, image.YCbCrSubsampleRatio410:
				for x, v := range ys {
					i := (x0+x)/4 - x0/4
					r, g, bl := ycbcr.RGB(v, cb[i], cr[i])
					*(*[4]byte)(outRow[4*x:]) = [4]byte{r, g, bl, 0xff}
				}
			default:
				for x, v := range ys {
					r, g, bl := ycbcr.RGB(v, cb[x], cr[x])
					*(*[4]byte)(outRow[4*x:]) = [4]byte{r, g, bl, 0xff}
				}
			}
		}

		return dst
	}

	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			dst = append(dst, c.R, c.G, c.B, c.A)
		}
	}

	return dst
}

// unpremultiply sets dst to the color.NRGBA that color.NRGBAModel returns
// for the color.RGBA src: each color c becomes c*0x101*0xffff/(a*0x101)>>8,
// which is c*0xffff/a>>8 since 0x101 divides out.
func unpremultiply(dst, src *[4]byte) {
	switch a := src[3]; a {
	case 0xff:
		*dst = *src
	case 0:
		*dst = [4]byte{}
	default:
		inv := reciprocals[a]
		for i := range 3 {
			dst[i] = uint8(quotient(uint32(src[i])*0xffff, inv) >> 8)
		}

		dst[3] = a
	}
}

// quotient returns n/a rounded down, for n below 2^24 and inv = 1/a with a
// below 256. The true quotient is an integer or at least 1/255 below the next
// one, and n*inv errs by less than 2^-28, so adding 2^-20 and truncating is
// exact.
func quotient(n uint32, inv float64) uint32 {
	return uint32(float64(float64(n)*inv) + 0x1p-20)
}

// allOpaque reports whether every pixel in the RGBA bytes pix has alpha 255.
func allOpaque(pix []byte) bool {
	for i := 3; i < len(pix); i += 4 {
		if pix[i] != 0xff {
			return false
		}
	}

	return true
}
