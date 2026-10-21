package thumbhash

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"math"
	"math/rand/v2"
	"sync"
	"testing"

	"github.com/superiority-dev/thumbhash/internal/codec"
	"github.com/superiority-dev/thumbhash/internal/format"
	"github.com/superiority-dev/thumbhash/internal/testimage"
)

// TestDecodeReference requires every decoded image, every value its bytes are
// truncated from, and every aspect ratio to equal the reference's bit for bit.
func TestDecodeReference(t *testing.T) {
	decodeRef, err := testimage.DecodeReference()
	if err != nil {
		t.Fatal(err)
	}

	for _, r := range decodeRef.Results {
		img, err := Decode(r.Hash)
		if err != nil {
			t.Errorf("%s: %v", r.ID, err)

			continue
		}

		if s := img.Bounds().Size(); s != image.Pt(r.W, r.H) {
			t.Errorf("%s: %v image, reference %dx%d", r.ID, s, r.W, r.H)
		}

		if !r.MatchesRGBA(img.Pix) {
			t.Errorf("%s: decoded bytes differ from the reference's", r.ID)
		}

		var buf [4 * DecodedSize * DecodedSize]float64

		if _, _, vals := codec.Render(&buf, r.Hash); !r.MatchesValues(vals) {
			t.Errorf("%s: values before truncation differ from the reference's", r.ID)
		}

		ratio, err := AspectRatio(r.Hash)
		if err != nil || math.Float64bits(ratio) != math.Float64bits(r.Ratio) {
			t.Errorf("%s: aspect ratio %v, %v; reference %v", r.ID, ratio, err, r.Ratio)
		}
	}
}

// TestDecodeConcurrent requires Decode to return the reference's images when
// called from several goroutines at once, none sharing memory with a later
// call's.
func TestDecodeConcurrent(t *testing.T) {
	decodeRef, err := testimage.DecodeReference()
	if err != nil {
		t.Fatal(err)
	}

	const goroutines = 8

	kept := make([][]*image.NRGBA, goroutines)

	var wg sync.WaitGroup

	for g := range goroutines {
		kept[g] = make([]*image.NRGBA, len(decodeRef.Results))

		wg.Go(func() {
			for _, i := range rand.New(rand.NewPCG(uint64(g), 0)).Perm(len(decodeRef.Results)) {
				img, err := Decode(decodeRef.Results[i].Hash)
				if err != nil {
					t.Errorf("%s: %v", decodeRef.Results[i].ID, err)
				}

				kept[g][i] = img
			}
		})
	}

	wg.Wait()

	for g := range goroutines {
		for i, r := range decodeRef.Results {
			if kept[g][i] == nil {
				continue
			}

			if !r.MatchesRGBA(kept[g][i].Pix) {
				t.Errorf("goroutine %d, %s: decoded bytes differ from the reference's", g, r.ID)
			}
		}
	}
}

// TestDecodeShort requires an error, not a panic, for every hash cut short,
// and DecodeInto to leave its destination unchanged.
func TestDecodeShort(t *testing.T) {
	decodeRef, err := testimage.DecodeReference()
	if err != nil {
		t.Fatal(err)
	}

	dst := filled(image.Rect(0, 0, DecodedSize, DecodedSize))
	before := bytes.Clone(dst.Pix)

	for _, r := range decodeRef.Results {
		for n := range len(r.Hash) {
			if img, err := Decode(r.Hash[:n]); !errors.Is(err, ErrInvalidHash) || img != nil {
				t.Fatalf("%s cut to %d bytes: got %v, %v; want nil and ErrInvalidHash", r.ID, n, img, err)
			}

			if _, err := DecodeInto(dst, r.Hash[:n]); !errors.Is(err, ErrInvalidHash) || !bytes.Equal(dst.Pix, before) {
				t.Fatalf("%s cut to %d bytes: DecodeInto returned %v or changed dst; want ErrInvalidHash", r.ID, n, err)
			}
		}
	}

	// Shorter than the five bytes every header has.
	for _, hash := range [][]byte{nil, {}, {1, 2, 3, 4}} {
		if _, err := AspectRatio(hash); !errors.Is(err, ErrInvalidHash) {
			t.Errorf("AspectRatio(%x): error %v, want ErrInvalidHash", hash, err)
		}
	}
}

// TestDecodeIntoMatchesDecode requires DecodeInto to write exactly what Decode
// returns, at dst's top left corner through dst's stride, and to leave every
// other pixel of dst alone.
func TestDecodeIntoMatchesDecode(t *testing.T) {
	decodeRef, err := testimage.DecodeReference()
	if err != nil {
		t.Fatal(err)
	}

	// Larger than DecodedSize and at an odd origin, one destination for every hash.
	dst := filled(image.Rect(3, 5, 3+40, 5+36))

	for _, res := range decodeRef.Results {
		want, err := Decode(res.Hash)
		if err != nil {
			t.Fatalf("%s: %v", res.ID, err)
		}

		r, err := DecodeInto(dst, res.Hash)
		if err != nil || r != want.Bounds().Add(dst.Rect.Min) {
			t.Fatalf("%s: DecodeInto returned %v, %v; want %v", res.ID, r, err, want.Bounds().Add(dst.Rect.Min))
		}

		for y := dst.Rect.Min.Y; y < dst.Rect.Max.Y; y++ {
			for x := dst.Rect.Min.X; x < dst.Rect.Max.X; x++ {
				got, inside := dst.NRGBAAt(x, y), image.Pt(x, y).In(r)
				if inside && got != want.NRGBAAt(x-r.Min.X, y-r.Min.Y) || !inside && got != sentinel {
					t.Fatalf("%s: pixel (%d, %d) is %v, inside %v", res.ID, x, y, got, inside)
				}
			}
		}

		copy(dst.Pix, filled(dst.Rect).Pix)
	}
}

// TestDecodeIntoConcurrent requires DecodeInto to write the reference's images
// when called from several goroutines at once, each into its own destination.
func TestDecodeIntoConcurrent(t *testing.T) {
	decodeRef, err := testimage.DecodeReference()
	if err != nil {
		t.Fatal(err)
	}

	const goroutines = 8

	var wg sync.WaitGroup

	for g := range goroutines {
		wg.Go(func() {
			dst := image.NewNRGBA(image.Rect(0, 0, DecodedSize, DecodedSize))
			pix := make([]byte, 0, len(dst.Pix))

			for _, i := range rand.New(rand.NewPCG(uint64(g), 0)).Perm(len(decodeRef.Results)) {
				res := decodeRef.Results[i]

				r, err := DecodeInto(dst, res.Hash)
				if err != nil {
					t.Errorf("goroutine %d, %s: %v", g, res.ID, err)

					continue
				}

				pix = pix[:0]
				for y := r.Min.Y; y < r.Max.Y; y++ {
					pix = append(pix, dst.Pix[dst.PixOffset(r.Min.X, y):][:4*r.Dx()]...)
				}

				if !res.MatchesRGBA(pix) {
					t.Errorf("goroutine %d, %s: decoded bytes differ from the reference's", g, res.ID)
				}
			}
		})
	}

	wg.Wait()
}

// TestDecodeIntoSmall requires ErrDestinationTooSmall for a destination under
// DecodedSize pixels on either side, whatever the hash, and dst left unchanged.
func TestDecodeIntoSmall(t *testing.T) {
	tooShort := make([]byte, format.MinLen)
	full := filled(image.Rect(0, 0, DecodedSize, DecodedSize))

	for _, r := range []image.Rectangle{
		image.Rect(0, 0, DecodedSize-1, DecodedSize),
		image.Rect(0, 0, DecodedSize, DecodedSize-1),
		{},
		{Min: image.Pt(math.MaxInt, 0), Max: image.Pt(-3, DecodedSize)}, // Min past Max: Size wraps to 2^63-2 by 32
	} {
		// Pixels for DecodedSize by DecodedSize, so only Rect can refuse.
		dst := &image.NRGBA{Pix: bytes.Clone(full.Pix), Stride: full.Stride, Rect: r}

		if _, err := DecodeInto(dst, tooShort); !errors.Is(err, ErrDestinationTooSmall) || !bytes.Equal(dst.Pix, full.Pix) {
			t.Errorf("%v destination: error %v or dst changed; want ErrDestinationTooSmall", r, err)
		}
	}
}

// TestDecodeIntoWide requires DecodeInto to write at the top left corner of a
// destination with a side past the int range, which Size wraps negative.
func TestDecodeIntoWide(t *testing.T) {
	decodeRef, err := testimage.DecodeReference()
	if err != nil {
		t.Fatal(err)
	}

	hash := decodeRef.Results[0].Hash

	want, err := Decode(hash)
	if err != nil {
		t.Fatal(err)
	}

	full := filled(image.Rect(0, 0, DecodedSize, DecodedSize))
	w := want.Rect.Dx()

	for _, r := range []image.Rectangle{
		{Min: image.Pt(math.MinInt, 0), Max: image.Pt(math.MaxInt, DecodedSize)},
		{Min: image.Pt(-2, 0), Max: image.Pt(math.MaxInt, DecodedSize)},
		{Min: image.Pt(0, math.MinInt), Max: image.Pt(DecodedSize, math.MaxInt)},
	} {
		// Pixels for the top left corner, all DecodeInto writes.
		dst := &image.NRGBA{Pix: bytes.Clone(full.Pix), Stride: full.Stride, Rect: r}

		got, err := DecodeInto(dst, hash)
		if err != nil || got != want.Rect.Add(r.Min) {
			t.Fatalf("%v destination: DecodeInto returned %v, %v; want %v", r, got, err, want.Rect.Add(r.Min))
		}

		for y := range want.Rect.Dy() {
			if !bytes.Equal(dst.Pix[y*dst.Stride:][:4*w], want.Pix[y*want.Stride:][:4*w]) {
				t.Fatalf("%v destination: row %d differs from Decode", r, y)
			}
		}
	}
}

func TestDecodeIntoAllocs(t *testing.T) {
	decodeRef, err := testimage.DecodeReference()
	if err != nil {
		t.Fatal(err)
	}

	dst := image.NewNRGBA(image.Rect(0, 0, DecodedSize, DecodedSize))
	hash := decodeRef.Results[0].Hash

	if n := testing.AllocsPerRun(20, func() { _, _ = DecodeInto(dst, hash) }); n != 0 {
		t.Errorf("%v allocations per DecodeInto, want 0", n)
	}
}

// FuzzDecode requires Decode, DecodeInto and AspectRatio to agree on which
// hashes they accept, and Decode and DecodeInto on the pixels.
func FuzzDecode(f *testing.F) {
	decodeRef, err := testimage.DecodeReference()
	if err != nil {
		f.Fatal(err)
	}

	for _, r := range decodeRef.Results {
		f.Add(r.Hash)
	}

	dst := image.NewNRGBA(image.Rect(0, 0, DecodedSize, DecodedSize))

	f.Fuzz(func(t *testing.T, hash []byte) {
		img, err := Decode(hash)
		r, errInto := DecodeInto(dst, hash)
		_, errRatio := AspectRatio(hash)

		if (err == nil) != (errInto == nil) || (errRatio == nil) != (len(hash) >= format.MinLen) {
			t.Fatalf("%x: errors %v, %v, %v", hash, err, errInto, errRatio)
		}

		if err != nil {
			if !errors.Is(err, ErrInvalidHash) {
				t.Fatalf("%x: %v, want ErrInvalidHash", hash, err)
			}

			return
		}

		if r != img.Rect || r.Dx() > DecodedSize || r.Dy() > DecodedSize {
			t.Fatalf("%x: Decode %v, DecodeInto %v", hash, img.Rect, r)
		}

		for y := range r.Dy() {
			if !bytes.Equal(img.Pix[img.PixOffset(0, y):][:4*r.Dx()], dst.Pix[dst.PixOffset(0, y):][:4*r.Dx()]) {
				t.Fatalf("%x: row %d differs between Decode and DecodeInto", hash, y)
			}
		}
	})
}

// sentinel fills every pixel of a destination before a decode, so a write
// outside the decoded region shows.
var sentinel = color.NRGBA{R: 0xab, G: 0xcd, B: 0xef, A: 0x12}

// filled returns an NRGBA image with bounds r, every pixel sentinel.
func filled(r image.Rectangle) *image.NRGBA {
	px := []byte{sentinel.R, sentinel.G, sentinel.B, sentinel.A}

	return &image.NRGBA{Pix: bytes.Repeat(px, r.Dx()*r.Dy()), Stride: 4 * r.Dx(), Rect: r}
}
