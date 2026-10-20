package thumbhash

import (
	"bytes"
	"errors"
	"image"
	"math"
	"testing"

	"github.com/superiority-dev/thumbhash/internal/codec"
	"github.com/superiority-dev/thumbhash/internal/testimage"
)

// TestEncodeReference requires every value the encoder rounds, and every hash,
// to equal the reference's bit for bit, and no hash Encode returns to share
// memory with a later call's.
func TestEncodeReference(t *testing.T) {
	ref, err := testimage.Reference()
	if err != nil {
		t.Fatal(err)
	}

	var enc Encoder

	cases := testimage.Cases()
	kept := make([][]byte, len(cases))

	for i, c := range cases {
		r := ref.Results[i]
		if r.ID != c.ID {
			t.Fatalf("result %d is %s, want %s", i, r.ID, c.ID)
		}

		img := c.Image()
		if kept[i], err = Encode(img); err != nil {
			t.Fatalf("%s: %v", c.ID, err)
		}

		s := img.Bounds().Size()
		vals, _ := enc.c.Unrounded(codec.Straight(nil, img), s.X, s.Y)

		if len(vals) != len(r.Unrounded) {
			t.Errorf("%s: %d values, reference has %d", c.ID, len(vals), len(r.Unrounded))

			continue
		}

		for j, v := range vals {
			if math.Float64bits(v) != math.Float64bits(r.Unrounded[j]) {
				t.Errorf("%s: value %d is %v, reference %v", c.ID, j, v, r.Unrounded[j])

				break
			}
		}

		hash, err := enc.Encode(img)
		if err != nil {
			t.Fatalf("%s: %v", c.ID, err)
		}

		if !bytes.Equal(hash, r.Hash) {
			t.Errorf("%s: hash %x, reference %x", c.ID, hash, r.Hash)
		}
	}

	for i, c := range cases {
		if !bytes.Equal(kept[i], ref.Results[i].Hash) {
			t.Errorf("%s: Encode returned %x, reference %x", c.ID, kept[i], ref.Results[i].Hash)
		}
	}
}

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
