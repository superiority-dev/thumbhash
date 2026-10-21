package codec

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/superiority-dev/thumbhash/internal/fdlibm"
	"github.com/superiority-dev/thumbhash/internal/format"
	"github.com/superiority-dev/thumbhash/internal/testimage"
)

// TestRenderMatchesSlow requires Render to equal slowRender bit for bit, on the
// decode fixture's hashes and on random ones.
func TestRenderMatchesSlow(t *testing.T) {
	decodeRef, err := testimage.DecodeReference()
	if err != nil {
		t.Fatal(err)
	}

	hashes := make([][]byte, 0, len(decodeRef.Results)+2000)
	for _, r := range decodeRef.Results {
		hashes = append(hashes, r.Hash)
	}

	r := rand.New(rand.NewPCG(3, 4))
	for len(hashes) < cap(hashes) {
		// 40 bytes is longer than any header requires.
		hash := make([]byte, 40)
		for i := range hash {
			hash[i] = uint8(r.Uint32())
		}

		hashes = append(hashes, hash)
	}

	var buf [4 * DecodedSize * DecodedSize]float64

	for _, hash := range hashes {
		gw, gh, got := Render(&buf, hash)
		ww, wh, want := slowRender(hash)

		if gw != ww || gh != wh || len(got) != len(want) {
			t.Fatalf("%x: %dx%d, slowRender %dx%d", hash, gw, gh, ww, wh)
		}

		if i := firstDiff(got, want); i >= 0 {
			t.Fatalf("%x: value %d is %v, slowRender %v", hash, i, got[i], want[i])
		}
	}
}

// slowRender is Render computed one pixel at a time, with its own cosines: the
// oracle Render is checked against.
func slowRender(hash []byte) (w, h int, vals []float64) {
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
	vals = make([]float64, 0, 4*w*h)

	for y := range h {
		for x := range w {
			l, p, q, a := lDC, pDC, qDC, aDC

			for cx := range nx {
				fx[cx] = fdlibm.Cos(math.Pi / float64(w) * (float64(x) + 0.5) * float64(cx))
			}

			for cy := range ny {
				fy[cy] = fdlibm.Cos(math.Pi / float64(h) * (float64(y) + 0.5) * float64(cy))
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
