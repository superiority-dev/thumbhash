package testimage

import (
	"image"
	"image/color"
	"image/draw"
)

const paletteSize = 64

var kinds = []imageKind{
	kindNRGBA, kindRGBA, kindRGBA64, kindNRGBA64, kindGray, kindGray16, kindYCbCr444,
	kindYCbCr420, kindNYCbCrA, kindPaletted, kindCMYK, kindAlpha, kindAlpha16, kindCustom,
}

// wide lists the kinds that store 16 bits per channel.
var wide = []imageKind{kindRGBA64, kindNRGBA64, kindGray16, kindAlpha16}

// imageKind is the type a test image is stored as: one of package image's
// pixel-buffer types, YCbCr at two subsampling ratios, or custom. Its name
// appears in case IDs, as in type/gray/opaque/33x41.
type imageKind uint8

const (
	kindNRGBA imageKind = iota
	kindRGBA
	kindRGBA64
	kindNRGBA64
	kindGray
	kindGray16
	kindYCbCr444
	kindYCbCr420
	kindNYCbCrA
	kindPaletted
	kindCMYK
	kindAlpha
	kindAlpha16
	kindCustom
)

func (k imageKind) String() string {
	return [...]string{
		"nrgba", "rgba", "rgba64", "nrgba64", "gray", "gray16", "ycbcr444",
		"ycbcr420", "nycbcra", "paletted", "cmyk", "alpha", "alpha16", "custom",
	}[k]
}

// alphaOnly reports whether k stores alpha and no color.
func (k imageKind) alphaOnly() bool {
	return k == kindAlpha || k == kindAlpha16
}

// carriesAlpha reports whether k stores alpha.
func (k imageKind) carriesAlpha() bool {
	switch k {
	case kindGray, kindGray16, kindYCbCr444, kindYCbCr420, kindCMYK:
		return false
	}

	return true
}

// custom hides an NRGBA behind an image type and a color type the encoder
// has no fast path for.
type custom struct {
	img *image.NRGBA
}

func (c custom) ColorModel() color.Model { return color.ModelFunc(toCustom) }

func (c custom) Bounds() image.Rectangle { return c.img.Bounds() }

func (c custom) At(x, y int) color.Color { return customColor{c.img.NRGBAAt(x, y)} }

type customColor struct {
	c color.NRGBA
}

func (c customColor) RGBA() (r, g, b, a uint32) { return c.c.RGBA() }

// convert stores src in an image of the given kind.
func convert(kind imageKind, src *image.NRGBA) image.Image {
	b := src.Bounds()

	switch kind {
	case kindNRGBA:
		return src
	case kindRGBA:
		return drawInto(image.NewRGBA(b), src)
	case kindRGBA64:
		return drawInto(image.NewRGBA64(b), src)
	case kindNRGBA64:
		return drawInto(image.NewNRGBA64(b), src)
	case kindGray:
		return drawInto(image.NewGray(b), src)
	case kindGray16:
		return drawInto(image.NewGray16(b), src)
	case kindYCbCr444:
		return ycbcrInto(image.NewYCbCr(b, image.YCbCrSubsampleRatio444), src)
	case kindYCbCr420:
		return ycbcrInto(image.NewYCbCr(b, image.YCbCrSubsampleRatio420), src)
	case kindNYCbCrA:
		dst := image.NewNYCbCrA(b, image.YCbCrSubsampleRatio444)
		ycbcrInto(&dst.YCbCr, src)

		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				dst.A[dst.AOffset(x, y)] = src.NRGBAAt(x, y).A
			}
		}

		return dst
	case kindPaletted:
		return drawInto(image.NewPaletted(b, palette(!src.Opaque())), src)
	case kindCMYK:
		return drawInto(image.NewCMYK(b), src)
	case kindAlpha:
		return drawInto(image.NewAlpha(b), src)
	case kindAlpha16:
		return drawInto(image.NewAlpha16(b), src)
	case kindCustom:
		return custom{src}
	}

	panic("testimage: unknown image kind")
}

// drawInto copies src into dst through dst's color model.
func drawInto(dst draw.Image, src *image.NRGBA) image.Image {
	b := src.Bounds()

	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			dst.Set(x, y, src.At(x, y))
		}
	}

	return dst
}

// ycbcrInto writes src's color into dst, ignoring alpha. Where chroma is
// subsampled, the last pixel written to a sample wins.
func ycbcrInto(dst *image.YCbCr, src *image.NRGBA) image.Image {
	b := src.Bounds()

	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := src.NRGBAAt(x, y)
			yy, cb, cr := color.RGBToYCbCr(c.R, c.G, c.B)
			dst.Y[dst.YOffset(x, y)] = yy
			dst.Cb[dst.COffset(x, y)] = cb
			dst.Cr[dst.COffset(x, y)] = cr
		}
	}

	return dst
}

func palette(withAlpha bool) color.Palette {
	r := newSplitMix(paletteSeed)
	p := make(color.Palette, paletteSize)

	for i := range p {
		c := color.NRGBA{R: r.byte(), G: r.byte(), B: r.byte(), A: 255}
		if withAlpha && i%2 == 1 {
			c.A = r.byte()
		}

		p[i] = c
	}

	return p
}

func toCustom(c color.Color) color.Color {
	if c, ok := c.(customColor); ok {
		return c
	}

	return customColor{color.NRGBAModel.Convert(c).(color.NRGBA)}
}
