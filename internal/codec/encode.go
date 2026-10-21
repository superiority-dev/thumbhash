package codec

import (
	"math"
	"slices"

	"github.com/superiority-dev/thumbhash/internal/format"
)

// maxRows is the most rows a column of coefficients has.
const maxRows = format.LumLong

// alphas holds, for each alpha byte a, a/255 and a/255/255: the pixel's alpha
// and the weight its color bytes are multiplied by.
var alphas = func() (t [256]struct{ alpha, weight float64 }) {
	for a := range t {
		t[a].alpha = float64(a) / 255
		t[a].weight = t[a].alpha / 255
	}

	return t
}()

// opaqueWeighted holds, for each color byte v, the value an opaque pixel
// contributes: 255/255/255 times v, as the reference computes it.
var opaqueWeighted = func() (t [256]float64) {
	for v := range t {
		t[v] = alphas[255].weight * float64(v)
	}

	return t
}()

// An Encoder holds the memory Unrounded reuses between calls. The zero value
// is ready to use. An Encoder should be used by one goroutine at a time.
type Encoder struct {
	// Pix is the straight RGBA of the image being encoded, for Straight to
	// fill and Unrounded to read.
	Pix []byte

	l, p, q, a []float64 // luminance, yellow-blue, red-green, alpha: one value a pixel
	fx, fy, ac []float64 // the cosines in use, and the AC coefficients so far
	acc        []float64 // one channel's coefficient sums
	vals       []float64 // the values Unrounded returns
}

// Unrounded computes each value the encoder rounds into a hash for the w by h
// straight RGBA image pix, and reports whether the image has alpha.
func (e *Encoder) Unrounded(pix []byte, w, h int) ([]float64, bool) {
	n := w * h
	e.l, e.p, e.q, e.a = grow(e.l, n), grow(e.p, n), grow(e.q, n), grow(e.a, n)

	hasAlpha := !allOpaque(pix)
	if hasAlpha {
		e.channelsAlpha(pix)
	} else {
		e.channelsOpaque(pix)
	}

	rx, ry, nx, ny := luminanceComponents(w, h, hasAlpha)
	vals := e.vals[:0]
	vals = append(vals, rx, ry)

	// The cosines the channels use: fx[x*cosStride+cx] and fy[cy*h+y].
	e.fx, e.fy = encodeCosines(w).x, encodeCosines(h).y

	e.ac = e.ac[:0]
	lDC, lScale := e.transform(e.l, w, h, nx, ny)
	pDC, pScale, qDC, qScale := e.transformPQ(e.p, e.q, w, h)

	// Scaled to each field's largest code: 63 for 6 bits, 31 for 5, 15 for 4.
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

// channelsOpaque fills e.l, e.p and e.q from pix, straight RGBA with every
// alpha 255.
func (e *Encoder) channelsOpaque(pix []byte) {
	for i := range len(pix) / 4 {
		r, g, b := opaqueWeighted[pix[4*i]], opaqueWeighted[pix[4*i+1]], opaqueWeighted[pix[4*i+2]]
		e.l[i] = (r + g + b) / 3
		e.p[i] = float64((r+g)/2) - b
		e.q[i] = r - g
	}
}

// channelsAlpha fills e.l, e.p, e.q and e.a from pix, straight RGBA, each
// pixel composited atop the image's average color, weighted by alpha.
func (e *Encoder) channelsAlpha(pix []byte) {
	var avgR, avgG, avgB, avgA float64
	for i := 0; i < len(pix); i += 4 {
		entry := &alphas[pix[i+3]]
		avgR += float64(entry.weight * float64(pix[i]))
		avgG += float64(entry.weight * float64(pix[i+1]))
		avgB += float64(entry.weight * float64(pix[i+2]))
		avgA += entry.alpha
	}

	if avgA != 0 {
		avgR /= avgA
		avgG /= avgA
		avgB /= avgA
	}

	for i := range len(pix) / 4 {
		j := 4 * i
		entry := &alphas[pix[j+3]]
		r := float64(entry.weight * float64(pix[j]))
		g := float64(entry.weight * float64(pix[j+1]))
		b := float64(entry.weight * float64(pix[j+2]))

		// An opaque pixel's blend would add avg*0, exactly 0.
		if pix[j+3] != 255 {
			r = float64(avgR*(1-entry.alpha)) + r
			g = float64(avgG*(1-entry.alpha)) + g
			b = float64(avgB*(1-entry.alpha)) + b
		}

		e.l[i] = (r + g + b) / 3
		e.p[i] = float64((r+g)/2) - b
		e.q[i] = r - g
		e.a[i] = entry.alpha
	}
}

// transform appends the normalized AC coefficients of ch, a w by h channel,
// at nx by ny components to e.ac, and returns its DC coefficient and AC
// scale. Each term is ch[i]*fx then times fy, summed in the reference's pixel
// order.
func (e *Encoder) transform(ch []float64, w, h, nx, ny int) (dc, scale float64) {
	var rows, at [maxRows]int

	n := staircase(nx, ny, &rows, &at)
	e.acc = grow(e.acc, n)

	for cx := 0; cx < nx; {
		if next := cx + 1; rows[cx] <= 4 && next < nx && rows[next] <= 3 {
			sa, sb := sumPair(ch, e.fx, e.fy, w, h, cx, rows[cx], rows[next])
			place(e.acc, &at, cx, rows[cx], &sa)
			place(e.acc, &at, next, rows[next], &sb)
			cx += 2

			continue
		}

		sa := sumColumn(ch, e.fx, e.fy, w, h, cx, rows[cx])
		place(e.acc, &at, cx, rows[cx], &sa)
		cx++
	}

	return e.normalize(e.acc, w*h)
}

// transformPQ is transform for the P and Q channels, both at format.PQSide
// components a side.
func (e *Encoder) transformPQ(p, q []float64, w, h int) (pDC, pScale, qDC, qScale float64) {
	var rows, at [maxRows]int

	n := staircase(format.PQSide, format.PQSide, &rows, &at)
	e.acc = grow(e.acc, 2*n)
	accP, accQ := e.acc[:n], e.acc[n:]

	for cx := range format.PQSide {
		sp, sq := sumPQ(p, q, e.fx, e.fy, w, h, cx, rows[cx])
		place(accP, &at, cx, rows[cx], &sp)
		place(accQ, &at, cx, rows[cx], &sq)
	}

	pDC, pScale = e.normalize(accP, w*h)
	qDC, qScale = e.normalize(accQ, w*h)

	return pDC, pScale, qDC, qScale
}

// normalize divides the sums in acc by the pixel count n, appends the AC
// coefficients normalized by their largest magnitude to e.ac, and returns the
// DC coefficient and that scale.
func (e *Encoder) normalize(acc []float64, n int) (dc, scale float64) {
	for k := range acc {
		acc[k] /= float64(n)
	}

	dc = acc[0]
	start := len(e.ac)

	for _, f := range acc[1:] {
		e.ac = append(e.ac, f)
		scale = max(scale, math.Abs(f))
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

// luminanceComponents returns luminance's component counts for a w by h image
// and the values they are rounded from: format.LumLong, or format.LumLongAlpha
// with alpha, times each side over the longer side. Each count is at least
// format.MinComponents.
func luminanceComponents(w, h int, hasAlpha bool) (rx, ry float64, nx, ny int) {
	limit := float64(format.LumLong)
	if hasAlpha {
		limit = format.LumLongAlpha
	}

	side := float64(max(w, h))
	rx, ry = limit*float64(w)/side, limit*float64(h)/side

	return rx, ry, max(format.MinComponents, int(math.Round(rx))), max(format.MinComponents, int(math.Round(ry)))
}

// staircase returns how many coefficients an nx by ny channel has, and sets
// rows[cx] to the number in column cx and at[cy] to the index of row cy's
// first, in the reference's order: cy outer, cx inner.
func staircase(nx, ny int, rows, at *[maxRows]int) int {
	n := 0
	for cy := range ny {
		at[cy] = n
		for cx := 0; cx*ny < nx*(ny-cy); cx++ {
			rows[cx]++
			n++
		}
	}

	return n
}

// place stores the sums of column cx's rows at their indices in acc.
func place(acc []float64, at *[maxRows]int, cx, rows int, sums *[maxRows]float64) {
	for r := range rows {
		acc[at[r]+cx] = sums[r]
	}
}

// grow returns s with length n, reusing its array when it is large enough.
func grow(s []float64, n int) []float64 {
	return slices.Grow(s[:0], n)[:n]
}
