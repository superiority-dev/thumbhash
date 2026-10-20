package fdlibm

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math"
	"testing"
)

// domainDigest is the SHA-256 of the big-endian bits of Cos at every argument
// [args] returns, in order. Every build target must reproduce it.
const domainDigest = "3f888717e4e5047c3a9617a482f09518b44b68d6f025514c647667496304d8c9"

// TestCos requires Cos to return the same bits on every build target, at every
// argument the format produces.
func TestCos(t *testing.T) {
	h := sha256.New()
	for _, x := range args() {
		h.Write(binary.BigEndian.AppendUint64(nil, math.Float64bits(Cos(x))))
	}

	if got := hex.EncodeToString(h.Sum(nil)); got != domainDigest {
		t.Errorf("cosines have digest %s, want %s", got, domainDigest)
	}
}

// args returns every cosine argument the format produces: the encoder's
// PI/n*c*(k+0.5) for n up to 100, then the decoder's PI/n*(k+0.5)*c for n up
// to 32, with c below 7.
func args() []float64 {
	var out []float64

	for n := 1; n <= 100; n++ {
		for c := range 7 {
			for k := range n {
				out = append(out, math.Pi/float64(n)*float64(c)*(float64(k)+0.5))
			}
		}
	}

	for n := 1; n <= 32; n++ {
		for c := range 7 {
			for k := range n {
				out = append(out, math.Pi/float64(n)*(float64(k)+0.5)*float64(c))
			}
		}
	}

	return out
}
