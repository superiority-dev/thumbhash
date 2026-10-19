package thumbhash

import (
	"errors"
	"image"
	"math"
	"testing"
)

// TestEncodeSize requires ErrEmptyImage for an image with no pixels, even when
// its other side is too large, ErrImageTooLarge past MaxSize, and no error at
// 1x1 and at MaxSize by MaxSize.
func TestEncodeSize(t *testing.T) {
	// The last four wrap in Size: Min past Max to 1x1, a side past the int range
	// negative.
	for _, c := range []struct {
		r    image.Rectangle
		want error
	}{
		{image.Rect(0, 0, 0, 0), ErrEmptyImage},
		{image.Rect(0, 0, 0, 5), ErrEmptyImage},
		{image.Rect(0, 0, 5, 0), ErrEmptyImage},
		{image.Rect(0, 0, 0, MaxSize+1), ErrEmptyImage},
		{image.Rect(0, 0, MaxSize+1, 0), ErrEmptyImage},
		{image.Rect(0, 0, MaxSize+1, 1), ErrImageTooLarge},
		{image.Rect(0, 0, 1, MaxSize+1), ErrImageTooLarge},
		{image.Rectangle{Min: image.Pt(math.MaxInt, 0), Max: image.Pt(math.MinInt, 1)}, ErrEmptyImage},
		{image.Rectangle{Min: image.Pt(math.MinInt, 0), Max: image.Pt(math.MaxInt, 1)}, ErrImageTooLarge},
		{image.Rectangle{Min: image.Pt(-2, 0), Max: image.Pt(math.MaxInt, 1)}, ErrImageTooLarge},
		{image.Rectangle{Min: image.Pt(0, math.MinInt), Max: image.Pt(1, math.MaxInt)}, ErrImageTooLarge},
	} {
		hash, err := Encode(&image.NRGBA{Rect: c.r})
		if !errors.Is(err, c.want) || hash != nil {
			t.Errorf("%v: got %x, %v; want nil and %v", c.r, hash, err, c.want)
		}
	}

	// The smallest and largest accepted sizes.
	for _, s := range []image.Point{{1, 1}, {MaxSize, MaxSize}} {
		if _, err := Encode(image.NewNRGBA(image.Rectangle{Max: s})); err != nil {
			t.Errorf("%v: %v", s, err)
		}
	}
}
