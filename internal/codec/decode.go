package codec

import (
	"math"

	"github.com/superiority-dev/thumbhash/internal/format"
)

// DecodedSize is the longer side of every image Render decodes.
const DecodedSize = 32

// Room for one channel's AC coefficients: L has at most 27, at 7 by 7, and A
// at most 14, at 5 by 5.
const acSlots, alphaACSlots = 32, 16

// coefficients holds one channel's AC coefficients: n of them, the values v at
// columns cx and rows cy.
type coefficients struct {
	n      int
	v      [acSlots]float64
	cx, cy [acSlots]uint8
}

func (c *coefficients) add(cx, cy int, v float64) {
	c.v[c.n], c.cx[c.n], c.cy[c.n] = v, uint8(cx), uint8(cy)
	c.n++
}

// Render computes the size of the image hash decodes to and the value each
// RGBA byte of it is truncated from, in row order, into dst. hash must be at
// least as long as its header says.
func Render(dst *[4 * DecodedSize * DecodedSize]float64, hash []byte) (w, h int, vals []float64) {
	hdr := format.Parse(hash)

	// Each field's code over its largest: 63 for 6 bits, 31 for 5, 15 for 4.
	lDC := float64(hdr.LDC) / 63
	pDC := float64(hdr.PDC)/31.5 - 1
	qDC := float64(hdr.QDC)/31.5 - 1
	lScale := float64(hdr.LScale) / 31
	pScale := float64(hdr.PScale) / 63
	qScale := float64(hdr.QScale) / 63

	lx, ly := hdr.Components()

	aDC, aScale := 1.0, 0.0
	start := hdr.Size()

	if hdr.Alpha {
		aDC = float64(hdr.ADC) / 15
		aScale = float64(hdr.AScale) / 15
	}

	// Each channel's AC coefficients in the reference's order, P and Q boosted
	// by 1.25 to compensate for quantization.
	var l, p, q, a coefficients

	index := 0
	read := func(c *coefficients, nx, ny int, scale float64) {
		for cy := range ny {
			for cx := firstAC(cy); cx*ny < nx*(ny-cy); cx++ {
				nibble := hash[start+index>>1] >> (4 * (index & 1)) & 15
				c.add(cx, cy, (float64(nibble)/7.5-1)*scale)
				index++
			}
		}
	}

	read(&l, lx, ly, lScale)
	read(&p, format.PQSide, format.PQSide, pScale*1.25)
	read(&q, format.PQSide, format.PQSide, qScale*1.25)

	if hdr.Alpha {
		read(&a, format.ASide, format.ASide, aScale)
	}

	ratio := hdr.AspectRatio()
	if ratio > 1 {
		w, h = DecodedSize, int(math.Round(DecodedSize/ratio))
	} else {
		w, h = int(math.Round(float64(DecodedSize*ratio))), DecodedSize
	}

	// The cosines a row per pixel: fx[x][cx], and fy[y][cy] already doubled.
	fx, fy := decodeCosines(w).x, decodeCosines(h).y

	// Each channel's coefficients as slices of one length.
	lval := l.v[:l.n]
	lcx, lcy := l.cx[:len(lval)], l.cy[:len(lval)]
	pval := p.v[:p.n]
	pcx, pcy, qval := p.cx[:len(pval)], p.cy[:len(pval)], q.v[:len(pval)]

	// A has at most alphaACSlots coefficients.
	aval := (*[alphaACSlots]float64)(a.v[:alphaACSlots])[:a.n]
	acx, acy := a.cx[:len(aval)], a.cy[:len(aval)]

	// For L and A, each coefficient times its cosine at each pixel column: the
	// product the reference rounds first.
	var lfxs [DecodedSize][acSlots]float64

	var afxs [DecodedSize][alphaACSlots]float64

	lfx, afx := lfxs[:len(fx)], afxs[:len(fx)]

	for x := range fx {
		cosX := &fx[x]

		row := lfx[x][:len(lval)]
		for j, v := range lval {
			row[j] = v * cosX[lcx[j]&cosMask]
		}

		arow := afx[x][:len(aval)]
		for j, v := range aval {
			arow[j] = v * cosX[acx[j]&cosMask]
		}
	}

	vals = dst[: 0 : 4*w*h]

	for y := range fy {
		cosY := &fy[y]

		// Four pixels at a time, each summing its terms in the reference's
		// order on its own accumulators.
		for x := 0; x < len(fx)-3; x += 4 {
			l0, l1, l2, l3 := lDC, lDC, lDC, lDC
			lfx0, lfx1 := lfx[x][:len(lcy)], lfx[x+1][:len(lcy)]
			lfx2, lfx3 := lfx[x+2][:len(lcy)], lfx[x+3][:len(lcy)]

			for j, cy := range lcy {
				cos := cosY[cy&cosMask]
				l0 += float64(lfx0[j] * cos)
				l1 += float64(lfx1[j] * cos)
				l2 += float64(lfx2[j] * cos)
				l3 += float64(lfx3[j] * cos)
			}

			p0, p1, p2, p3 := pDC, pDC, pDC, pDC
			q0, q1, q2, q3 := qDC, qDC, qDC, qDC
			cosX0, cosX1, cosX2, cosX3 := &fx[x], &fx[x+1], &fx[x+2], &fx[x+3]

			for j, v := range pval {
				cx, cos := pcx[j]&cosMask, cosY[pcy[j]&cosMask]
				xy0, xy1, xy2, xy3 := float64(cosX0[cx]*cos), float64(cosX1[cx]*cos), float64(cosX2[cx]*cos), float64(cosX3[cx]*cos)
				p0 += float64(v * xy0)
				p1 += float64(v * xy1)
				p2 += float64(v * xy2)
				p3 += float64(v * xy3)
				q0 += float64(qval[j] * xy0)
				q1 += float64(qval[j] * xy1)
				q2 += float64(qval[j] * xy2)
				q3 += float64(qval[j] * xy3)
			}

			a0, a1, a2, a3 := aDC, aDC, aDC, aDC
			afx0, afx1 := afx[x][:len(acy)], afx[x+1][:len(acy)]
			afx2, afx3 := afx[x+2][:len(acy)], afx[x+3][:len(acy)]

			for j, cy := range acy {
				cos := cosY[cy&cosMask]
				a0 += float64(afx0[j] * cos)
				a1 += float64(afx1[j] * cos)
				a2 += float64(afx2[j] * cos)
				a3 += float64(afx3[j] * cos)
			}

			vals = appendRGBA(vals, l0, p0, q0, a0)
			vals = appendRGBA(vals, l1, p1, q1, a1)
			vals = appendRGBA(vals, l2, p2, q2, a2)
			vals = appendRGBA(vals, l3, p3, q3, a3)
		}

		// The pixels left over, one at a time.
		for x := len(fx) &^ 3; x < len(fx); x++ {
			cosX := &fx[x]
			lv, pv, qv, av := lDC, pDC, qDC, aDC

			for j, lf := range lfx[x][:len(lcy)] {
				lv += float64(lf * cosY[lcy[j]&cosMask])
			}

			for j, v := range pval {
				xy := float64(cosX[pcx[j]&cosMask] * cosY[pcy[j]&cosMask])
				pv += float64(v * xy)
				qv += float64(qval[j] * xy)
			}

			for j, af := range afx[x][:len(acy)] {
				av += float64(af * cosY[acy[j]&cosMask])
			}

			vals = appendRGBA(vals, lv, pv, qv, av)
		}
	}

	return w, h, vals
}

// appendRGBA appends the values the bytes of a pixel with the given L, P, Q
// and A are truncated from: L, P and Q converted to R, G and B, then each
// channel and A clamped to [0, 1] and scaled to 255.
func appendRGBA(vals []float64, l, p, q, a float64) []float64 {
	b := l - float64(2.0/3*p)
	r := float64((float64(3*l) - b + q) / 2)
	g := r - q

	return append(vals,
		max(0, 255*min(1, r)),
		max(0, 255*min(1, g)),
		max(0, 255*min(1, b)),
		max(0, 255*min(1, a)))
}

// firstAC is the first cx with an AC coefficient in row cy; row 0 starts with
// the DC term.
func firstAC(cy int) int {
	if cy == 0 {
		return 1
	}

	return 0
}
