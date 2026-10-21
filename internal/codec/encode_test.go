package codec

import (
	"bytes"
	"fmt"
	"image"
	"iter"
	"math"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/superiority-dev/thumbhash/internal/fdlibm"
	"github.com/superiority-dev/thumbhash/internal/format"
	"github.com/superiority-dev/thumbhash/internal/testimage"
)

// TestUnroundedEverySize requires Unrounded to run without a panic at every
// accepted size, opaque and with alpha, through the full kernels.
func TestUnroundedEverySize(t *testing.T) {
	if raceEnabled {
		t.Skip("skipped under -race; TestTransformPairs covers the kernel dispatch")
	}

	opaque := bytes.Repeat([]byte{0, 0, 0, 255}, format.MaxSize*format.MaxSize)
	transparent := make([]byte, 4*format.MaxSize*format.MaxSize)

	var e Encoder

	for w := 1; w <= format.MaxSize; w++ {
		for h := 1; h <= format.MaxSize; h++ {
			for _, pix := range [][]byte{opaque, transparent} {
				func() {
					defer func() {
						if r := recover(); r != nil {
							t.Fatalf("%dx%d, alpha %v: %v", w, h, pix[3] != 255, r)
						}
					}()

					e.Unrounded(pix[:4*w*h], w, h)
				}()
			}
		}
	}
}

// TestUnroundedMatchesSlow requires Unrounded to equal slowUnrounded bit for
// bit, on the test images and on random ones.
func TestUnroundedMatchesSlow(t *testing.T) {
	// Each side keeps its Encoder across images, as a caller reuses one.
	var fast, slow Encoder

	check := func(id string, img image.Image) {
		s := img.Bounds().Size()
		pix := Straight(nil, img)
		got, gotAlpha := fast.Unrounded(pix, s.X, s.Y)
		want, wantAlpha := slow.slowUnrounded(pix, s.X, s.Y)

		if gotAlpha != wantAlpha || len(got) != len(want) {
			t.Errorf("%s: %d values, alpha %v; slowUnrounded %d, %v", id, len(got), gotAlpha, len(want), wantAlpha)

			return
		}

		if i := firstDiff(got, want); i >= 0 {
			t.Errorf("%s: value %d is %v, slowUnrounded %v", id, i, got[i], want[i])
		}
	}

	for _, c := range testimage.Cases() {
		check(c.ID, c.Image())
	}

	for id, img := range randomImages(400) {
		check(id, img)
	}
}

// TestSlowUnroundedLeavesTables requires slowUnrounded to leave the shared
// cosine tables alone.
func TestSlowUnroundedLeavesTables(t *testing.T) {
	for _, id := range []string{"noise/opaque/8x8", "noise/alpha-any/9x7"} {
		c, ok := testimage.Lookup(id)
		if !ok {
			t.Fatalf("no case %s", id)
		}

		img := c.Image()
		s := img.Bounds().Size()
		pix := Straight(nil, img)

		var e Encoder

		first, _ := e.Unrounded(pix, s.X, s.Y)
		first = slices.Clone(first)

		e.slowUnrounded(pix, s.X, s.Y)

		again, _ := e.Unrounded(pix, s.X, s.Y)
		if len(again) != len(first) {
			t.Fatalf("%s: %d values after slowUnrounded, %d before", id, len(again), len(first))
		}

		if i := firstDiff(again, first); i >= 0 {
			t.Fatalf("%s: value %d is %v after slowUnrounded, %v before", id, i, again[i], first[i])
		}
	}
}

// TestTransformPairs requires transform to have a kernel for every pair of
// columns it sums in one pass: at the luminance layout of every accepted size,
// opaque and with alpha, and at alpha's layout.
func TestTransformPairs(t *testing.T) {
	// Each distinct layout, with the first size that gets it.
	layouts := map[[2]int]string{{format.ASide, format.ASide}: "alpha"}

	for w := 1; w <= format.MaxSize; w++ {
		for h := 1; h <= format.MaxSize; h++ {
			for _, hasAlpha := range []bool{false, true} {
				_, _, nx, ny := luminanceComponents(w, h, hasAlpha)
				if _, ok := layouts[[2]int{nx, ny}]; !ok {
					layouts[[2]int{nx, ny}] = fmt.Sprintf("%dx%d, alpha %v", w, h, hasAlpha)
				}
			}
		}
	}

	var e Encoder

	for l, from := range layouts {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("%s, %d by %d components: %v", from, l[0], l[1], r)
				}
			}()

			e.fx, e.fy = encodeCosines(1).x, encodeCosines(1).y
			e.ac = e.ac[:0]
			e.transform([]float64{0}, 1, 1, l[0], l[1])
		}()
	}
}

// randomImages yields n seeded images of random sizes up to format.MaxSize
// square, each named for its alpha: opaque, any, 0 or 255, or one of 0, 1,
// 254 and 255.
func randomImages(n int) iter.Seq2[string, *image.NRGBA] {
	alphas := [...]string{"opaque", "alpha-any", "alpha-binary", "alpha-0-1-254-255"}

	return func(yield func(string, *image.NRGBA) bool) {
		r := rand.New(rand.NewPCG(1, 2))

		for i := range n {
			img := image.NewNRGBA(image.Rect(0, 0, 1+r.IntN(format.MaxSize), 1+r.IntN(format.MaxSize)))

			// PCG has no Read; only ChaCha8 does.
			for j := range img.Pix {
				img.Pix[j] = uint8(r.Uint32())
			}

			for j := 3; j < len(img.Pix); j += 4 {
				switch alphas[i%4] {
				case "opaque":
					img.Pix[j] = 255
				case "alpha-binary":
					img.Pix[j] = []uint8{0, 255}[img.Pix[j]>>7]
				case "alpha-0-1-254-255":
					img.Pix[j] = []uint8{0, 1, 254, 255}[img.Pix[j]&3]
				}
			}

			if !yield(fmt.Sprintf("random/%s/%dx%d", alphas[i%4], img.Rect.Dx(), img.Rect.Dy()), img) {
				return
			}
		}
	}
}

// slowUnrounded is Unrounded computed one coefficient at a time over every
// pixel: the oracle Unrounded is checked against.
func (e *Encoder) slowUnrounded(pix []byte, w, h int) ([]float64, bool) {
	n := w * h

	// Average color, weighted by alpha.
	var avgR, avgG, avgB, avgA float64
	for i := 0; i < len(pix); i += 4 {
		alpha := float64(pix[i+3]) / 255
		avgR += float64(alpha / 255 * float64(pix[i]))
		avgG += float64(alpha / 255 * float64(pix[i+1]))
		avgB += float64(alpha / 255 * float64(pix[i+2]))
		avgA += alpha
	}

	if avgA != 0 {
		avgR /= avgA
		avgG /= avgA
		avgB /= avgA
	}

	hasAlpha := avgA < float64(n)
	limit := float64(format.LumLong)
	if hasAlpha {
		limit = format.LumLongAlpha
	}

	side := float64(max(w, h))
	vals := e.vals[:0]
	vals = append(vals, limit*float64(w)/side, limit*float64(h)/side)
	lx := max(1, int(math.Round(vals[0])))
	ly := max(1, int(math.Round(vals[1])))

	// Composite atop the average color and convert to LPQA.
	e.l, e.p, e.q, e.a = grow(e.l, n), grow(e.p, n), grow(e.q, n), grow(e.a, n)
	for i := range n {
		j := 4 * i
		alpha := float64(pix[j+3]) / 255
		r := float64(avgR*(1-alpha)) + float64(alpha/255*float64(pix[j]))
		g := float64(avgG*(1-alpha)) + float64(alpha/255*float64(pix[j+1]))
		b := float64(avgB*(1-alpha)) + float64(alpha/255*float64(pix[j+2]))
		e.l[i] = (r + g + b) / 3
		e.p[i] = float64((r+g)/2) - b
		e.q[i] = r - g
		e.a[i] = alpha
	}

	e.ac = e.ac[:0]
	lDC, lScale := e.slowTransform(e.l, w, h, max(format.MinComponents, lx), max(format.MinComponents, ly))
	pDC, pScale := e.slowTransform(e.p, w, h, format.PQSide, format.PQSide)
	qDC, qScale := e.slowTransform(e.q, w, h, format.PQSide, format.PQSide)

	vals = append(vals, 63*lDC, 31.5+float64(31.5*pDC), 31.5+float64(31.5*qDC), 31*lScale, 63*pScale, 63*qScale)

	if hasAlpha {
		aDC, aScale := e.slowTransform(e.a, w, h, format.ASide, format.ASide)
		vals = append(vals, 15*aDC, 15*aScale)
	}

	for _, f := range e.ac {
		vals = append(vals, 15*f)
	}

	e.vals = vals

	return vals, hasAlpha
}

// slowTransform is transform computed one coefficient at a time over every
// pixel, for slowUnrounded.
func (e *Encoder) slowTransform(ch []float64, w, h, nx, ny int) (dc, scale float64) {
	start := len(e.ac)
	fx := make([]float64, w)

	for cy := range ny {
		for cx := 0; cx*ny < nx*(ny-cy); cx++ {
			var f float64

			for x := range w {
				fx[x] = fdlibm.Cos(math.Pi / float64(w) * float64(cx) * (float64(x) + 0.5))
			}

			for y := range h {
				fy := fdlibm.Cos(math.Pi / float64(h) * float64(cy) * (float64(y) + 0.5))
				for x := range w {
					f += float64(ch[x+y*w] * fx[x] * fy)
				}
			}

			f /= float64(w * h)

			if cx != 0 || cy != 0 {
				e.ac = append(e.ac, f)
				scale = max(scale, math.Abs(f))
			} else {
				dc = f
			}
		}
	}

	if scale != 0 {
		for i := start; i < len(e.ac); i++ {
			e.ac[i] = 0.5 + float64(0.5/scale*e.ac[i])
		}
	}

	return dc, scale
}
