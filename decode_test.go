package thumbhash

import (
	"errors"
	"image"
	"math"
	"math/rand/v2"
	"sync"
	"testing"

	"github.com/superiority-dev/thumbhash/internal/codec"
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

// TestDecodeShort requires an error, not a panic, for every hash cut short.
func TestDecodeShort(t *testing.T) {
	decodeRef, err := testimage.DecodeReference()
	if err != nil {
		t.Fatal(err)
	}

	for _, r := range decodeRef.Results {
		for n := range len(r.Hash) {
			if img, err := Decode(r.Hash[:n]); !errors.Is(err, ErrInvalidHash) || img != nil {
				t.Fatalf("%s cut to %d bytes: got %v, %v; want nil and ErrInvalidHash", r.ID, n, img, err)
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
