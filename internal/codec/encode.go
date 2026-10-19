package codec

import (
	"math"
	"slices"

	"github.com/superiority-dev/thumbhash/internal/format"
)

// An Encoder holds the memory Unrounded reuses between calls. The zero value
// is ready to use. An Encoder should be used by one goroutine at a time.
type Encoder struct {
	// Pix is the straight RGBA of the image being encoded, for Straight to
	// fill and Unrounded to read.
	Pix []byte

	l, p, q, a []float64 // luminance, yellow-blue, red-green, alpha: one value a pixel
	ac         []float64 // the AC coefficients so far
	vals       []float64 // the values Unrounded returns
}

// Unrounded computes each value the encoder rounds into a hash for the w by h
// straight RGBA image pix, and reports whether the image has alpha.
func (e *Encoder) Unrounded(pix []byte, w, h int) ([]float64, bool) {
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
	lDC, lScale := e.transform(e.l, w, h, max(format.MinComponents, lx), max(format.MinComponents, ly))
	pDC, pScale := e.transform(e.p, w, h, format.PQSide, format.PQSide)
	qDC, qScale := e.transform(e.q, w, h, format.PQSide, format.PQSide)

	vals = append(vals, 63*lDC, 31.5+float64(31.5*pDC), 31.5+float64(31.5*qDC), 31*lScale, 63*pScale, 63*qScale)

	if hasAlpha {
		aDC, aScale := e.transform(e.a, w, h, format.ASide, format.ASide)
		vals = append(vals, 15*aDC, 15*aScale)
	}

	for _, f := range e.ac {
		vals = append(vals, 15*f)
	}

	e.vals = vals

	return vals, hasAlpha
}

// transform appends the normalized AC coefficients of ch, a w by h channel,
// at nx by ny components to e.ac, and returns its DC coefficient and AC
// scale. Each coefficient is one pass over every pixel, in the reference's
// order.
func (e *Encoder) transform(ch []float64, w, h, nx, ny int) (dc, scale float64) {
	start := len(e.ac)
	fx := make([]float64, w)

	for cy := range ny {
		for cx := 0; cx*ny < nx*(ny-cy); cx++ {
			var f float64

			for x := range w {
				fx[x] = math.Cos(math.Pi / float64(w) * float64(cx) * (float64(x) + 0.5))
			}

			for y := range h {
				fy := math.Cos(math.Pi / float64(h) * float64(cy) * (float64(y) + 0.5))
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

// Quantize rounds and packs the values Unrounded returns for a w by h image
// into a hash: a five-byte header, a sixth byte for alpha, then the AC values
// four bits each, two to a byte, low half first.
func Quantize(vals []float64, w, h int, hasAlpha bool) []byte {
	round := func(i int) int { return int(math.Round(vals[i])) }

	isLandscape := w > h

	// The header stores the luminance component count of the shorter side.
	count := max(1, round(0))
	if isLandscape {
		count = max(1, round(1))
	}

	hdr := format.Header{
		LDC: uint8(round(2)), PDC: uint8(round(3)), QDC: uint8(round(4)), LScale: uint8(round(5)),
		PScale: uint8(round(6)), QScale: uint8(round(7)), Count: uint8(count),
		Alpha: hasAlpha, Landscape: isLandscape,
	}

	ac := vals[8:]
	if hasAlpha {
		hdr.ADC, hdr.AScale, ac = uint8(round(8)), uint8(round(9)), vals[10:]
	}

	n := hdr.Size() + (len(ac)+1)/2
	hash := hdr.Append(make([]byte, 0, n))[:n]

	for i, f := range ac {
		hash[hdr.Size()+i/2] |= byte(int(math.Round(f)) << (4 * (i & 1)))
	}

	return hash
}

// grow returns s with length n, reusing its array when it is large enough.
func grow(s []float64, n int) []float64 {
	return slices.Grow(s[:0], n)[:n]
}
