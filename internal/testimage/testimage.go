// Package testimage builds the test images and holds what the reference
// encoder and decoder produced for them. Each image is a pure function of its
// ID.
package testimage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"slices"
)

// Fixed seeds for the random generator. The values are arbitrary; changing one
// changes the test cases built from it, so the fixtures must be regenerated.
const (
	noise16Seed = 0x16b17
	offsetSeed  = 0x0ff5e7
	paletteSeed = 0xb1a5
	headerSeed  = 0xdec0de
)

var (
	// From 1x1 to 100x100, the reference encoder's limit: odd and even,
	// square, wide and tall.
	sizes = []image.Point{
		{1, 1}, {1, 2}, {2, 1}, {2, 2}, {3, 7}, {7, 3}, {8, 8}, {9, 7},
		{16, 16}, {17, 31}, {31, 17}, {32, 24}, {24, 32}, {33, 41}, {64, 48},
		{1, 100}, {100, 1}, {75, 100}, {100, 75}, {99, 100}, {100, 100},
	}
	// Smallest, even, odd, and 100x75.
	solidSizes = []image.Point{{1, 1}, {8, 8}, {9, 7}, {100, 75}}

	// Smallest that splits both ways, odd, and 100x75.
	splitSizes = []image.Point{{2, 2}, {9, 7}, {100, 75}}

	// One column and one row at the 100-pixel limit, split one way only.
	lineSizes = []image.Point{{1, 100}, {100, 1}}

	// A spread of sizes, from a single column to the limit, for each alpha
	// pattern.
	alphaSizes = []image.Point{{1, 100}, {2, 2}, {3, 7}, {9, 7}, {17, 31}, {32, 24}, {100, 75}, {100, 100}}

	// Smallest with a ramp both ways, up to the limit.
	rampSizes = []image.Point{{2, 2}, {9, 7}, {17, 31}, {32, 24}, {100, 75}, {100, 100}}

	// Odd, and 100x75, for each image type.
	typeSizes = []image.Point{{33, 41}, {100, 75}}

	// Sizes where 7 or, with alpha, 5 times the shorter side over the longer
	// is exactly 2.5 or 4.5, which half-to-even rounding takes to 2 and 4.
	opaqueTies = []image.Point{{5, 14}, {14, 5}, {9, 14}}
	alphaTies  = []image.Point{{1, 2}, {2, 1}, {9, 10}}

	// Black and white and one step in from each, the grays either side of the
	// midpoint, the three primaries, yellow, a green and a near-black.
	solids = []color.NRGBA{
		{0, 0, 0, 255}, {1, 1, 1, 255}, {127, 127, 127, 255}, {128, 128, 128, 255},
		{254, 254, 254, 255}, {255, 255, 255, 255}, {10, 220, 130, 255}, {255, 0, 0, 255},
		{0, 255, 0, 255}, {0, 0, 255, 255}, {255, 255, 0, 255}, {3, 7, 11, 255},
	}

	// Translucent solids, including transparent pixels with and without RGB.
	translucent = []color.NRGBA{
		{10, 220, 130, 0}, {10, 220, 130, 1}, {10, 220, 130, 127},
		{10, 220, 130, 128}, {10, 220, 130, 254}, {0, 0, 0, 0},
	}

	// Black against white, the two grays either side of the midpoint, white
	// against one step below it, and red against blue.
	opaquePairs = [][2]color.NRGBA{
		{gray(0), gray(255)}, {gray(127), gray(128)}, {gray(254), gray(255)},
		{{255, 0, 0, 255}, {0, 0, 255, 255}},
	}

	// A binary mask with RGB under alpha 0, and partial alpha.
	alphaPairs = [][2]color.NRGBA{
		{{255, 255, 255, 255}, {200, 40, 90, 0}},
		{{255, 0, 255, 64}, {0, 255, 255, 192}},
	}

	// Translucent splits in which a fused multiply-add on arm64 moves a live
	// coefficient across a rounding boundary.
	fusionSplits = []struct {
		p        [2]color.NRGBA
		s        image.Point
		vertical bool
	}{
		{[2]color.NRGBA{{105, 202, 166, 56}, {232, 247, 39, 90}}, image.Pt(2, 2), false},
		{[2]color.NRGBA{{105, 202, 166, 56}, {232, 247, 39, 90}}, image.Pt(2, 2), true},
		{[2]color.NRGBA{{41, 177, 133, 123}, {111, 200, 201, 199}}, image.Pt(9, 7), false},
		{[2]color.NRGBA{{41, 177, 133, 123}, {111, 200, 201, 199}}, image.Pt(9, 7), true},
		{[2]color.NRGBA{{153, 69, 188, 50}, {99, 183, 142, 233}}, image.Pt(32, 24), false},
		{[2]color.NRGBA{{153, 69, 188, 50}, {99, 183, 142, 233}}, image.Pt(32, 24), true},
	}

	// The kinds the encoder reads directly, plus paletted, each drawn into a
	// sub-image.
	offsets = []struct {
		kind  imageKind
		alpha alphaMode
	}{
		{kindNRGBA, opaque}, {kindRGBA, opaque}, {kindYCbCr444, opaque},
		{kindPaletted, opaque}, {kindNRGBA, alphaAny}, {kindRGBA, alphaAny},
	}

	// The alpha modes the noise family covers.
	alphas = []alphaMode{alphaAny, alphaBinary, alpha1or254}

	// One size each for the families that cover how an image is stored rather
	// than its size: premultiplied translucent solids, 16-bit noise and
	// sub-images.
	premultipliedSize = image.Pt(8, 8)
	wideSize          = image.Pt(33, 41)
	offsetSize        = image.Pt(16, 16)
)

// Case is one test image. Its ID is "<family>/<variant>/<width>x<height>",
// and the variant may itself contain slashes.
type Case struct {
	ID    string
	build func() image.Image
}

// Image returns a new copy of the case's image on every call.
func (c Case) Image() image.Image {
	return c.build()
}

// Lookup returns the case with the given ID.
func Lookup(id string) (Case, bool) {
	cases := Cases()
	if i := slices.IndexFunc(cases, func(c Case) bool { return c.ID == id }); i >= 0 {
		return cases[i], true
	}

	return Case{}, false
}

// Fingerprint returns the SHA-256 of every case ID, size and reference input.
func Fingerprint() string {
	h := sha256.New()

	for _, c := range Cases() {
		img := c.Image()
		fmt.Fprintf(h, "%s %s\n", c.ID, size(img.Bounds().Size()))
		h.Write(Straight(img))
	}

	return hex.EncodeToString(h.Sum(nil))
}

// Cases returns every test image in a fixed order. The type, offset and
// solid-rgba families repeat earlier inputs: the reference file is made from
// [Straight], so only the fingerprint guards the conversions they cover.
func Cases() []Case {
	var out []Case

	add := func(id string, build func() image.Image) {
		out = append(out, Case{ID: id, build: build})
	}

	for _, c := range slices.Concat(solids, translucent) {
		for _, s := range solidSizes {
			add(fmt.Sprintf("solid/%s/%s", rgba(c), size(s)), solid(s, c))
		}
	}

	// At alpha 0 these repeat solid/0.0.0.0.
	for _, c := range translucent {
		s := premultipliedSize
		add(fmt.Sprintf("solid-rgba/%s/%s", rgba(c), size(s)), premultiplied(s, c))
	}

	for _, p := range opaquePairs {
		for _, s := range slices.Concat(splitSizes, lineSizes) {
			splits(add, p, s)
		}
	}

	for _, p := range alphaPairs {
		for _, s := range splitSizes {
			splits(add, p, s)
		}
	}

	for _, f := range fusionSplits {
		dir := "x"
		if f.vertical {
			dir = "y"
		}

		add(fmt.Sprintf("split-%s/%s-%s/%s", dir, rgba(f.p[0]), rgba(f.p[1]), size(f.s)), split(f.s, f.p, f.vertical))
	}

	for _, s := range sizes {
		// A 1x1 ramp is a solid.
		if s == image.Pt(1, 1) {
			continue
		}

		add("gradient/opaque/"+size(s), graded(s, nil))
	}

	for _, s := range rampSizes {
		add("gradient/alpha-x/"+size(s), graded(s, alphaAcross(s)))
		add("gradient/alpha-y/"+size(s), graded(s, alphaDown(s)))
	}

	for _, s := range sizes {
		add("noise/opaque/"+size(s), noisy(s, opaque))
	}

	for _, a := range alphas {
		for _, s := range alphaSizes {
			add("noise/"+a.String()+"/"+size(s), noisy(s, a))
		}
	}

	for _, s := range opaqueTies {
		add("tie/opaque/"+size(s), noisy(s, opaque))
	}

	for _, s := range alphaTies {
		add("tie/alpha-any/"+size(s), noisy(s, alphaAny))
	}

	for _, s := range typeSizes {
		for _, kind := range kinds {
			if !kind.alphaOnly() {
				add(fmt.Sprintf("type/%s/opaque/%s", kind, size(s)), typed(kind, s, opaque))
			}

			if kind.carriesAlpha() {
				add(fmt.Sprintf("type/%s/alpha/%s", kind, size(s)), typed(kind, s, alphaAny))
			}
		}
	}

	for _, kind := range wide {
		s := wideSize
		add(fmt.Sprintf("type/%s/noise16/%s", kind, size(s)), noise16(kind, s))
	}

	for _, o := range offsets {
		s := offsetSize
		add(fmt.Sprintf("offset/%s/%s/%s", o.kind, o.alpha, size(s)), offset(o.kind, s, o.alpha))
	}

	return out
}

// Straight returns img's pixels in row order as the 8-bit straight RGBA that
// [color.NRGBAModel] converts each one to. That is the input the reference
// encoder takes.
func Straight(img image.Image) []byte {
	b := img.Bounds()
	out := make([]byte, 0, 4*b.Dx()*b.Dy())

	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			out = append(out, c.R, c.G, c.B, c.A)
		}
	}

	return out
}

// splits adds p split across s left from right and top from bottom; a split
// across a single column or row is a solid, so it is left out.
func splits(add func(string, func() image.Image), p [2]color.NRGBA, s image.Point) {
	name := rgba(p[0]) + "-" + rgba(p[1])

	if s.X > 1 {
		add(fmt.Sprintf("split-x/%s/%s", name, size(s)), split(s, p, false))
	}

	if s.Y > 1 {
		add(fmt.Sprintf("split-y/%s/%s", name, size(s)), split(s, p, true))
	}
}

func size(p image.Point) string {
	return fmt.Sprintf("%dx%d", p.X, p.Y)
}

func rgba(c color.NRGBA) string {
	return fmt.Sprintf("%d.%d.%d.%d", c.R, c.G, c.B, c.A)
}
