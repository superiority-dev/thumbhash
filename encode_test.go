package thumbhash

import (
	"bytes"
	"errors"
	"image"
	"math"
	"math/rand/v2"
	"sync"
	"testing"

	"github.com/superiority-dev/thumbhash/internal/codec"
	"github.com/superiority-dev/thumbhash/internal/format"
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

// TestEncodeConcurrent requires Encode to return the reference's hashes when
// called from several goroutines at once.
func TestEncodeConcurrent(t *testing.T) {
	ref, err := testimage.Reference()
	if err != nil {
		t.Fatal(err)
	}

	cases := testimage.Cases()

	const goroutines = 8

	var wg sync.WaitGroup

	for g := range goroutines {
		wg.Go(func() {
			for _, i := range rand.New(rand.NewPCG(uint64(g), 0)).Perm(len(cases)) {
				hash, err := Encode(cases[i].Image())
				if err != nil || !bytes.Equal(hash, ref.Results[i].Hash) {
					t.Errorf("%s: Encode returns %x, %v; reference %x", cases[i].ID, hash, err, ref.Results[i].Hash)

					return
				}
			}
		})
	}

	wg.Wait()
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

func TestEncodeAllocs(t *testing.T) {
	// An opaque NRGBA and a translucent RGBA, each read by its own fast path.
	for _, id := range []string{"type/nrgba/opaque/100x75", "type/rgba/alpha/100x75"} {
		c, ok := testimage.Lookup(id)
		if !ok {
			t.Fatalf("no case %s", id)
		}

		img := c.Image()

		var e Encoder
		if _, err := e.Encode(img); err != nil {
			t.Fatalf("%s: %v", id, err)
		}

		if n := testing.AllocsPerRun(20, func() { _, _ = e.Encode(img) }); n != 1 {
			t.Errorf("%s: %v allocations per Encode, want 1", id, n)
		}
	}
}

// FuzzEncode requires Encode to accept exactly the sizes up to MaxSize, to
// return a hash Decode accepts, and to read NRGBA and RGBA as At does, at any
// origin.
func FuzzEncode(f *testing.F) {
	f.Add(int8(0), int8(0), uint8(1), uint8(1), []byte{1, 2, 3, 255})
	f.Add(int8(-3), int8(5), uint8(100), uint8(75), []byte{200, 100, 50, 255, 0, 0, 0, 0, 255, 255, 255, 128})
	f.Add(int8(0), int8(0), uint8(0), uint8(5), []byte{0})
	f.Add(int8(0), int8(0), uint8(101), uint8(1), []byte{0})

	f.Fuzz(func(t *testing.T, x0, y0 int8, w, h uint8, pix []byte) {
		r := image.Rect(int(x0), int(y0), int(x0)+int(w), int(y0)+int(h))

		// The fuzzed bytes, repeated to fill the image.
		buf := make([]byte, 4*r.Dx()*r.Dy())
		if len(pix) > 0 {
			for i := range buf {
				buf[i] = pix[i%len(pix)]
			}
		}

		for _, img := range []image.Image{
			&image.NRGBA{Pix: buf, Stride: 4 * r.Dx(), Rect: r},
			&image.RGBA{Pix: buf, Stride: 4 * r.Dx(), Rect: r},
		} {
			hash, err := Encode(img)
			if ok := w > 0 && h > 0 && w <= MaxSize && h <= MaxSize; (err == nil) != ok {
				t.Fatalf("%T %v: %v", img, r, err)
			}

			if err != nil {
				continue
			}

			if n := format.Parse(hash).HashLen(); n != len(hash) {
				t.Fatalf("%T %v: hash %x is %d bytes, its header says %d", img, r, hash, len(hash), n)
			}

			if _, err := Decode(hash); err != nil {
				t.Fatalf("%T %v: %v", img, r, err)
			}

			if at, _ := Encode(atOnly{img}); !bytes.Equal(hash, at) {
				t.Fatalf("%T %v: %x, through At %x", img, r, hash, at)
			}
		}
	})
}

// atOnly hides an image's type, so Encode reads it through At.
type atOnly struct{ image.Image }
