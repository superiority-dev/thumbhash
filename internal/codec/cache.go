package codec

import (
	"math"
	"sync/atomic"

	"github.com/superiority-dev/thumbhash/internal/fdlibm"
	"github.com/superiority-dev/thumbhash/internal/format"
)

// A pixel's cosines along x take cosStride slots: format.LumLong of them,
// padded to a power of two so cosMask keeps any index inside the row.
const (
	cosStride = 8
	cosMask   = cosStride - 1
)

// The cosines of each side length, built on first use and then shared. A
// table is stored only once complete; when two calls build the same one, both
// use the first stored.
var (
	encodeCache [format.MaxSize + 1]atomic.Pointer[cosines]
	decodeCache [DecodedSize + 1]atomic.Pointer[decodeTables]
)

// cosines holds one side length's cosines for format.LumLong components, x
// padded to cosStride a pixel.
type cosines struct{ x, y []float64 }

// decodeTables holds one side length's decoder cosines, a row per pixel.
type decodeTables struct {
	x, y [][cosStride]float64
}

// encodeCosines returns the encoder's cosines for side n: x[i*cosStride+c] and
// y[c*n+i] are both the cosine of math.Pi/n*c*(i+0.5), multiplied in that
// order.
func encodeCosines(n int) *cosines {
	if t := encodeCache[n].Load(); t != nil {
		return t
	}

	t := &cosines{x: make([]float64, cosStride*n), y: make([]float64, format.LumLong*n)}
	for i := range n {
		for c := range format.LumLong {
			v := fdlibm.Cos(math.Pi / float64(n) * float64(c) * (float64(i) + 0.5))
			t.x[i*cosStride+c], t.y[c*n+i] = v, v
		}
	}

	if !encodeCache[n].CompareAndSwap(nil, t) {
		t = encodeCache[n].Load()
	}

	return t
}

// decodeCosines returns the decoder's cosines for side n: x[i][c] is the
// cosine of math.Pi/n*(i+0.5)*c, multiplied in that order, and y[i][c] is
// twice it.
func decodeCosines(n int) *decodeTables {
	if t := decodeCache[n].Load(); t != nil {
		return t
	}

	t := &decodeTables{x: make([][cosStride]float64, n), y: make([][cosStride]float64, n)}
	for i := range n {
		for c := range format.LumLong {
			v := fdlibm.Cos(math.Pi / float64(n) * (float64(i) + 0.5) * float64(c))
			t.x[i][c], t.y[i][c] = v, v*2
		}
	}

	if !decodeCache[n].CompareAndSwap(nil, t) {
		t = decodeCache[n].Load()
	}

	return t
}
