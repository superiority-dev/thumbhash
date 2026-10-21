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

// TestStraightYCbCr requires the YCbCr read to match color.NRGBAModel where
// the test images do not reach: every subsample ratio, bounds starting at odd
// and negative coordinates and near both ends of the int range, and sub-images
// at odd offsets, all with random planes.
func TestStraightYCbCr(t *testing.T) {
	r := rand.New(rand.NewPCG(7, 8))

	// Every subsample ratio package image defines.
	ratios := []image.YCbCrSubsampleRatio{
		image.YCbCrSubsampleRatio444, image.YCbCrSubsampleRatio422, image.YCbCrSubsampleRatio420,
		image.YCbCrSubsampleRatio440, image.YCbCrSubsampleRatio411, image.YCbCrSubsampleRatio410,
	}

	// Bounds starting at zero, at odd and at negative coordinates, and near
	// both ends of the int range. image.NewYCbCr adds to Max, so none ends at
	// math.MaxInt.
	mins := []image.Point{
		{0, 0}, {1, 1}, {3, 5}, {-1, 0}, {-3, -5}, {-6, 7},
		{math.MinInt, math.MinInt}, {math.MinInt + 1, math.MinInt + 3},
		{math.MaxInt - 64, math.MaxInt - 64}, {math.MaxInt - 65, math.MaxInt - 67},
	}

	// Odd and even, from one pixel up, and one with no width.
	sizes := []image.Point{{1, 1}, {2, 2}, {5, 3}, {7, 9}, {32, 24}, {33, 41}, {0, 5}}

	for _, ratio := range ratios {
		for _, m := range mins {
			for _, s := range sizes {
				img := image.NewYCbCr(image.Rectangle{m, m.Add(s)}, ratio)
				for _, plane := range [][]byte{img.Y, img.Cb, img.Cr} {
					for i := range plane {
						plane[i] = byte(r.Uint32())
					}
				}

				checkStraight(t, fmt.Sprintf("%v %v", ratio, img.Rect), img)

				// Each plane one byte short at the end of its last row, the
				// byte still in capacity: slicing checks capacity, At's index
				// checks length.
				for _, cut := range []string{"Y", "Cb", "Cr"} {
					short := *img
					plane := map[string]*[]byte{"Y": &short.Y, "Cb": &short.Cb, "Cr": &short.Cr}[cut]
					if len(*plane) == 0 {
						continue
					}

					*plane = (*plane)[:len(*plane)-1]
					checkStraight(t, fmt.Sprintf("%v %v short %s", ratio, img.Rect, cut), &short)
				}

				// Sub-images starting one column, one row, or more in.
				for _, d := range []image.Point{{1, 0}, {0, 1}, {1, 1}, {3, 2}} {
					if sub := img.Rect; s.X > d.X && s.Y > d.Y {
						sub.Min = sub.Min.Add(d)
						checkStraight(t, fmt.Sprintf("%v %v sub-image %v", ratio, img.Rect, sub), img.SubImage(sub))
					}
				}
			}
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
