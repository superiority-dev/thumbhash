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

	// The cosines: fx[x*cosStride+cx], and fy[y*cosStride+cy] already doubled.
	fx, fy := decodeCosines(w).x, decodeCosines(h).y

	// For L and A, each coefficient times its cosine at each pixel column: the
	// product the reference rounds first.
	var lfx [DecodedSize * acSlots]float64

	var afx [DecodedSize * alphaACSlots]float64

	for x := range w {
		cosX := (*[cosStride]float64)(fx[x*cosStride : x*cosStride+cosStride])
		for j := range l.n {
			lfx[x*acSlots+j] = l.v[j] * cosX[l.cx[j]&cosMask]
		}

		for j := range a.n {
			afx[x*alphaACSlots+j] = a.v[j] * cosX[a.cx[j]&cosMask]
		}
	}

	vals = dst[: 0 : 4*w*h]

	for y := range h {
		cosY := (*[cosStride]float64)(fy[y*cosStride : y*cosStride+cosStride])

		// Four pixels at a time, each summing its terms in the reference's
		// order on its own accumulators.
		x := 0
		for ; x+4 <= w; x += 4 {
			l0, l1, l2, l3 := lDC, lDC, lDC, lDC
			lfx0, lfx1 := lfx[x*acSlots:][:l.n], lfx[(x+1)*acSlots:][:l.n]
			lfx2, lfx3 := lfx[(x+2)*acSlots:][:l.n], lfx[(x+3)*acSlots:][:l.n]

			for j := range l.n {
				cos := cosY[l.cy[j]&cosMask]
				l0 += float64(lfx0[j] * cos)
				l1 += float64(lfx1[j] * cos)
				l2 += float64(lfx2[j] * cos)
				l3 += float64(lfx3[j] * cos)
			}

			p0, p1, p2, p3 := pDC, pDC, pDC, pDC
			q0, q1, q2, q3 := qDC, qDC, qDC, qDC
			cosX0, cosX1 := (*[cosStride]float64)(fx[x*cosStride:x*cosStride+cosStride]), (*[cosStride]float64)(fx[(x+1)*cosStride:(x+1)*cosStride+cosStride])
			cosX2, cosX3 := (*[cosStride]float64)(fx[(x+2)*cosStride:(x+2)*cosStride+cosStride]), (*[cosStride]float64)(fx[(x+3)*cosStride:(x+3)*cosStride+cosStride])

			for j := range p.n {
				cx, cos := p.cx[j]&cosMask, cosY[p.cy[j]&cosMask]
				xy0, xy1, xy2, xy3 := float64(cosX0[cx]*cos), float64(cosX1[cx]*cos), float64(cosX2[cx]*cos), float64(cosX3[cx]*cos)
				p0 += float64(p.v[j] * xy0)
				p1 += float64(p.v[j] * xy1)
				p2 += float64(p.v[j] * xy2)
				p3 += float64(p.v[j] * xy3)
				q0 += float64(q.v[j] * xy0)
				q1 += float64(q.v[j] * xy1)
				q2 += float64(q.v[j] * xy2)
				q3 += float64(q.v[j] * xy3)
			}

			a0, a1, a2, a3 := aDC, aDC, aDC, aDC
			afx0, afx1 := afx[x*alphaACSlots:][:a.n], afx[(x+1)*alphaACSlots:][:a.n]
			afx2, afx3 := afx[(x+2)*alphaACSlots:][:a.n], afx[(x+3)*alphaACSlots:][:a.n]

			for j := range a.n {
				cos := cosY[a.cy[j]&cosMask]
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

		for ; x < w; x++ {
			cosX := (*[cosStride]float64)(fx[x*cosStride : x*cosStride+cosStride])
			lv, pv, qv, av := lDC, pDC, qDC, aDC

			for j, lf := range lfx[x*acSlots : x*acSlots+l.n] {
				lv += float64(lf * cosY[l.cy[j]&cosMask])
			}

			for j := range p.n {
				xy := float64(cosX[p.cx[j]&cosMask] * cosY[p.cy[j]&cosMask])
				pv += float64(p.v[j] * xy)
				qv += float64(q.v[j] * xy)
			}

			for j, af := range afx[x*alphaACSlots : x*alphaACSlots+a.n] {
				av += float64(af * cosY[a.cy[j]&cosMask])
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
