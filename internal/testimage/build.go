package testimage

import (
	"encoding/binary"
	"image"
	"image/color"
)

// The offset family tests images whose bounds do not start at (0, 0). Each is
// cut from a bigger noise image: it starts at offsetMin, and the bigger image
// extends offsetMargin past its right and bottom edges.
var offsetMin, offsetMargin = image.Pt(5, 3), image.Pt(4, 4)

// alphaMode is how a test image draws each pixel's alpha. Its name appears in
// case IDs, as in noise/alpha-any/32x24.
type alphaMode uint8

const (
	opaque alphaMode = iota
	alphaAny
	alphaBinary
	alpha1or254
)

func (m alphaMode) String() string {
	return [...]string{"opaque", "alpha-any", "alpha-binary", "alpha-1-254"}[m]
}

// pick draws one pixel's alpha from r; opaque draws nothing.
func (m alphaMode) pick(r *splitMix) uint8 {
	switch m {
	case alphaAny:
		return r.byte()
	case alphaBinary:
		return 255 * (r.byte() & 1)
	case alpha1or254:
		return 1 + 253*(r.byte()&1)
	}

	return 255
}

func solid(s image.Point, c color.NRGBA) func() image.Image {
	return func() image.Image { return fill(s, flat(c)) }
}

// premultiplied builds an s-sized image of c stored as premultiplied RGBA.
func premultiplied(s image.Point, c color.NRGBA) func() image.Image {
	return func() image.Image { return convert(kindRGBA, fill(s, flat(c))) }
}

// split builds p[0] over the first half of s and p[1] over the rest, split
// top from bottom if vertical and left from right otherwise.
func split(s image.Point, p [2]color.NRGBA, vertical bool) func() image.Image {
	return func() image.Image {
		return fill(s, func(x, y int) color.NRGBA {
			i, n := x, s.X
			if vertical {
				i, n = y, s.Y
			}

			if i < n/2 {
				return p[0]
			}

			return p[1]
		})
	}
}

// graded builds an s-sized gradient with the given alpha, or opaque if nil.
func graded(s image.Point, alpha func(x, y int) uint8) func() image.Image {
	return func() image.Image { return fill(s, gradient(s, alpha)) }
}

// alphaAcross ramps alpha from 0 to 255 left to right across s.
func alphaAcross(s image.Point) func(x, y int) uint8 {
	return func(x, _ int) uint8 { return ramp(x, s.X) }
}

// alphaDown ramps alpha from 0 to 255 top to bottom across s.
func alphaDown(s image.Point) func(x, y int) uint8 {
	return func(_, y int) uint8 { return ramp(y, s.Y) }
}

// noisy builds an s-sized image of uniform noise seeded by its size.
func noisy(s image.Point, a alphaMode) func() image.Image {
	return func() image.Image { return fill(s, noise(sizeSeed(s), a)) }
}

// typed builds the noisy image of the same size and alpha stored as kind.
func typed(kind imageKind, s image.Point, a alphaMode) func() image.Image {
	return func() image.Image { return convert(kind, fill(s, noise(sizeSeed(s), a))) }
}

// noise16 builds an s-sized image of the given 16-bit kind holding uniform
// 16-bit noise.
func noise16(kind imageKind, s image.Point) func() image.Image {
	return func() image.Image {
		r := newSplitMix(noise16Seed)
		b := image.Rectangle{Max: s}

		switch kind {
		case kindRGBA64:
			img := image.NewRGBA64(b)

			// Premultiplied, so no color channel exceeds alpha.
			for i := 0; i < len(img.Pix); i += 8 {
				a := r.word()
				for c := range 3 {
					binary.BigEndian.PutUint16(img.Pix[i+2*c:], uint16(uint32(r.word())%(uint32(a)+1)))
				}

				binary.BigEndian.PutUint16(img.Pix[i+6:], a)
			}

			return img
		case kindNRGBA64:
			img := image.NewNRGBA64(b)
			r.read(img.Pix)

			return img
		case kindGray16:
			img := image.NewGray16(b)
			r.read(img.Pix)

			return img
		case kindAlpha16:
			img := image.NewAlpha16(b)
			r.read(img.Pix)

			return img
		}

		panic("testimage: noise16 of an 8-bit kind")
	}
}

// offset builds an s-sized noise sub-image of the given kind.
func offset(kind imageKind, s image.Point, a alphaMode) func() image.Image {
	return func() image.Image {
		sub := image.Rectangle{Min: offsetMin, Max: offsetMin.Add(s)}
		full := convert(kind, fill(sub.Max.Add(offsetMargin), noise(offsetSeed, a)))

		return full.(interface {
			SubImage(image.Rectangle) image.Image
		}).SubImage(sub)
	}
}

func fill(s image.Point, at func(x, y int) color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rectangle{Max: s})

	for y := range s.Y {
		for x := range s.X {
			img.SetNRGBA(x, y, at(x, y))
		}
	}

	return img
}

func flat(c color.NRGBA) func(int, int) color.NRGBA {
	return func(int, int) color.NRGBA { return c }
}

func noise(seed uint64, a alphaMode) func(int, int) color.NRGBA {
	r := newSplitMix(seed)

	return func(int, int) color.NRGBA {
		return color.NRGBA{R: r.byte(), G: r.byte(), B: r.byte(), A: a.pick(r)}
	}
}

// gradient ramps red across, green down and blue along the diagonal of s.
// A nil alpha means opaque.
func gradient(s image.Point, alpha func(x, y int) uint8) func(int, int) color.NRGBA {
	return func(x, y int) color.NRGBA {
		c := color.NRGBA{R: ramp(x, s.X), G: ramp(y, s.Y), B: ramp(x+y, s.X+s.Y-1), A: 255}
		if alpha != nil {
			c.A = alpha(x, y)
		}

		return c
	}
}

func gray(v uint8) color.NRGBA {
	return color.NRGBA{R: v, G: v, B: v, A: 255}
}

// ramp maps i in [0, n) onto [0, 255].
func ramp(i, n int) uint8 {
	if n <= 1 {
		return 0
	}

	return uint8(i * 255 / (n - 1))
}

func sizeSeed(s image.Point) uint64 {
	return uint64(s.X)<<8 | uint64(s.Y)
}
