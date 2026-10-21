package codec

import (
	"bytes"
	"image"
	"math/rand/v2"
	"sync"
	"testing"

	"github.com/superiority-dev/thumbhash/internal/testimage"
)

// TestCacheConcurrent requires the reference's results while goroutines build
// and read the same cosine tables at once.
func TestCacheConcurrent(t *testing.T) {
	ref, err := testimage.Reference()
	if err != nil {
		t.Fatal(err)
	}

	decodeRef, err := testimage.DecodeReference()
	if err != nil {
		t.Fatal(err)
	}

	// Tables are kept per side length, so one image and one hash per size
	// reach every table the test data reaches.
	var encodes, decodes []int

	seen := map[image.Point]bool{}

	cases := testimage.Cases()

	images := make([]image.Image, len(cases))
	for i, c := range cases {
		images[i] = c.Image()
		if s := images[i].Bounds().Size(); !seen[s] {
			seen[s] = true
			encodes = append(encodes, i)
		}
	}

	clear(seen)

	for i, r := range decodeRef.Results {
		if s := image.Pt(r.W, r.H); !seen[s] {
			seen[s] = true
			decodes = append(decodes, i)
		}
	}

	const goroutines, rounds, maxReported = 8, 50, 5

	for pass := range rounds {
		// clear cannot reset an atomic.Pointer.
		for i := range encodeCache {
			encodeCache[i].Store(nil)
		}

		for i := range decodeCache {
			decodeCache[i].Store(nil)
		}

		hashes := make([][][]byte, goroutines)
		pix := make([][][]byte, goroutines)
		start := make(chan struct{})

		var wg sync.WaitGroup

		for g := range goroutines {
			hashes[g] = make([][]byte, len(cases))
			pix[g] = make([][]byte, len(decodeRef.Results))

			wg.Go(func() {
				<-start

				r := rand.New(rand.NewPCG(uint64(pass), uint64(g)))

				var enc Encoder

				for _, j := range r.Perm(len(encodes)) {
					i := encodes[j]
					s := images[i].Bounds().Size()
					enc.Pix = Straight(enc.Pix[:0], images[i])
					vals, hasAlpha := enc.Unrounded(enc.Pix, s.X, s.Y)
					hashes[g][i] = Quantize(vals, s.X, s.Y, hasAlpha)
				}

				var buf [4 * DecodedSize * DecodedSize]float64

				for _, j := range r.Perm(len(decodes)) {
					i := decodes[j]
					_, _, vals := Render(&buf, decodeRef.Results[i].Hash)

					pix[g][i] = make([]byte, len(vals))
					for k, v := range vals {
						pix[g][i][k] = uint8(v)
					}
				}
			})
		}

		close(start)
		wg.Wait()

		wrong := 0

		for g := range goroutines {
			for _, i := range encodes {
				if c := cases[i]; !bytes.Equal(hashes[g][i], ref.Results[i].Hash) {
					wrong++

					if wrong <= maxReported {
						t.Errorf("pass %d, goroutine %d, %s: hash %x, reference %x", pass, g, c.ID, hashes[g][i], ref.Results[i].Hash)
					}
				}
			}

			for _, i := range decodes {
				r := decodeRef.Results[i]
				if pix[g][i] == nil || !r.MatchesRGBA(pix[g][i]) {
					wrong++

					if wrong <= maxReported {
						t.Errorf("pass %d, goroutine %d, %s: decoded bytes differ from the reference's", pass, g, r.ID)
					}
				}
			}
		}

		if wrong > 0 {
			t.Fatalf("pass %d: %d of %d results wrong", pass, wrong, goroutines*(len(encodes)+len(decodes)))
		}
	}
}
