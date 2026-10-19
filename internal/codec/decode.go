package codec

import (
	"math"

	"github.com/superiority-dev/thumbhash/internal/format"
)

// DecodedSize is the longer side of every image Render decodes.
const DecodedSize = 32

// Render computes the size of the image hash decodes to and the value each
// RGBA byte of it is truncated from, in row order, into dst. hash must be at
// least as long as its header says.
func Render(dst *[4 * DecodedSize * DecodedSize]float64, hash []byte) (w, h int, vals []float64) {
	hdr := format.Parse(hash)

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

	// Each channel's AC coefficients, P and Q boosted by 1.25 to compensate for
	// quantization.
	index := 0
	channel := func(nx, ny int, scale float64) []float64 {
		var ac []float64

		for cy := range ny {
			for cx := firstAC(cy); cx*ny < nx*(ny-cy); cx++ {
				nibble := hash[start+index>>1] >> (4 * (index & 1)) & 15
				ac = append(ac, (float64(nibble)/7.5-1)*scale)
				index++
			}
		}

		return ac
	}

	lAC := channel(lx, ly, lScale)
	pAC := channel(format.PQSide, format.PQSide, pScale*1.25)
	qAC := channel(format.PQSide, format.PQSide, qScale*1.25)

	var aAC []float64
	if hdr.Alpha {
		aAC = channel(format.ASide, format.ASide, aScale)
	}

	ratio := hdr.AspectRatio()
	if ratio > 1 {
		w, h = DecodedSize, int(math.Round(DecodedSize/ratio))
	} else {
		w, h = int(math.Round(float64(DecodedSize*ratio))), DecodedSize
	}

	nx, ny := max(lx, format.PQSide), max(ly, format.PQSide)
	if hdr.Alpha {
		nx, ny = max(lx, format.ASide), max(ly, format.ASide)
	}

	fx, fy := make([]float64, nx), make([]float64, ny)
	vals = dst[: 0 : 4*w*h]

	for y := range h {
		for x := range w {
			l, p, q, a := lDC, pDC, qDC, aDC

			for cx := range nx {
				fx[cx] = math.Cos(math.Pi / float64(w) * (float64(x) + 0.5) * float64(cx))
			}

			for cy := range ny {
				fy[cy] = math.Cos(math.Pi / float64(h) * (float64(y) + 0.5) * float64(cy))
			}

			for cy, j := 0, 0; cy < ly; cy++ {
				fy2 := fy[cy] * 2
				for cx := firstAC(cy); cx*ly < lx*(ly-cy); cx++ {
					l += float64(lAC[j] * fx[cx] * fy2)
					j++
				}
			}

			for cy, j := 0, 0; cy < format.PQSide; cy++ {
				fy2 := fy[cy] * 2
				for cx := firstAC(cy); cx < format.PQSide-cy; cx++ {
					f := fx[cx] * fy2
					p += float64(pAC[j] * f)
					q += float64(qAC[j] * f)
					j++
				}
			}

			if hdr.Alpha {
				for cy, j := 0, 0; cy < format.ASide; cy++ {
					fy2 := fy[cy] * 2
					for cx := firstAC(cy); cx < format.ASide-cy; cx++ {
						a += float64(aAC[j] * fx[cx] * fy2)
						j++
					}
				}
			}

			b := l - float64(2.0/3*p)
			r := float64((float64(3*l) - b + q) / 2)
			g := r - q

			vals = append(vals,
				max(0, 255*min(1, r)),
				max(0, 255*min(1, g)),
				max(0, 255*min(1, b)),
				max(0, 255*min(1, a)))
		}
	}

	return w, h, vals
}

// firstAC is the first cx with an AC coefficient in row cy; row 0 starts with
// the DC term.
func firstAC(cy int) int {
	if cy == 0 {
		return 1
	}

	return 0
}
