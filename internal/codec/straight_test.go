package codec

import (
	"bytes"
	"fmt"
	"image"
	"math"
	"math/rand/v2"
	"testing"

	"github.com/superiority-dev/thumbhash/internal/testimage"
)

// TestStraightMatchesModel requires Straight to read every test image as
// color.NRGBAModel does.
func TestStraightMatchesModel(t *testing.T) {
	for _, c := range testimage.Cases() {
		img := c.Image()
		if got, want := Straight(nil, img), testimage.Straight(img); !bytes.Equal(got, want) {
			t.Errorf("%s: straight RGBA differs from color.NRGBAModel", c.ID)
		}
	}
}

// TestStraightIntLimits requires the NRGBA and RGBA reads to match At with
// bounds at both ends of the int range.
func TestStraightIntLimits(t *testing.T) {
	r := rand.New(rand.NewPCG(9, 10))
	s := image.Pt(7, 5)

	pix := make([]byte, 4*s.X*s.Y)
	for i := range pix {
		pix[i] = byte(r.Uint32())
	}

	for _, m := range []image.Point{{math.MinInt, math.MinInt}, image.Pt(math.MaxInt, math.MaxInt).Sub(s)} {
		b := image.Rectangle{m, m.Add(s)}
		checkStraight(t, fmt.Sprintf("NRGBA %v", b), &image.NRGBA{Pix: pix, Stride: 4 * s.X, Rect: b})
		checkStraight(t, fmt.Sprintf("RGBA %v", b), &image.RGBA{Pix: pix, Stride: 4 * s.X, Rect: b})
	}
}

// TestUnpremultiply requires the RGBA read to match color.NRGBAModel at every
// color value and alpha, colors above alpha included.
func TestUnpremultiply(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 256, 256))
	for c := range 256 {
		for a := range 256 {
			i := img.PixOffset(c, a)
			copy(img.Pix[i:], []byte{uint8(c), uint8(255 - c), uint8(c / 2), uint8(a)})
		}
	}

	got, want := Straight(nil, img), testimage.Straight(img)
	for i := 0; i < len(want); i += 4 {
		if !bytes.Equal(got[i:i+4], want[i:i+4]) {
			t.Fatalf("pixel %d: %v, color.NRGBAModel gives %v", i/4, got[i:i+4], want[i:i+4])
		}
	}
}

// checkStraight requires Straight to read img as color.NRGBAModel does, and
// to panic exactly when At does. image.NewYCbCr sizes the chroma planes with
// truncating division, so some bounds starting at negative odd coordinates
// get planes too small and At panics.
func checkStraight(t *testing.T, name string, img image.Image) {
	t.Helper()

	want, wantPanic := catch(func() []byte { return testimage.Straight(img) })
	got, gotPanic := catch(func() []byte { return Straight(nil, img) })

	switch {
	case gotPanic != wantPanic:
		t.Errorf("%s: Straight panicked %v, color.NRGBAModel %v", name, gotPanic, wantPanic)
	case !bytes.Equal(got, want):
		t.Errorf("%s: straight RGBA differs from color.NRGBAModel", name)
	}
}

// catch returns f's result, or reports that it panicked.
func catch(f func() []byte) (out []byte, panicked bool) {
	defer func() {
		if recover() != nil {
			panicked = true
		}
	}()

	return f(), false
}
