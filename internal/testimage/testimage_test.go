package testimage

import (
	"bytes"
	"math"
	"strings"
	"testing"

	"github.com/superiority-dev/thumbhash/internal/format"
)

// TestLookup requires Lookup to find every case by its ID, and nothing for an
// ID no case has.
func TestLookup(t *testing.T) {
	for _, want := range Cases() {
		if c, ok := Lookup(want.ID); !ok || c.ID != want.ID {
			t.Errorf("Lookup(%q) = %q, %v", want.ID, c.ID, ok)
		}
	}

	if _, ok := Lookup("noise/opaque/101x1"); ok {
		t.Error("found a case that does not exist")
	}
}

// TestReferenceMatchesCases requires reference.txt to be the reference's
// output for today's cases: the same fingerprint, count and IDs in order.
func TestReferenceMatchesCases(t *testing.T) {
	ref, err := Reference()
	if err != nil {
		t.Fatal(err)
	}

	if got, want := Fingerprint(), ref.Value("fingerprint"); got != want {
		t.Fatalf("images have fingerprint %s, but reference.txt is the reference's output for images with fingerprint %s; its header names the reference and the runtime that wrote it", got, want)
	}

	cases := Cases()
	if len(ref.Results) != len(cases) {
		t.Fatalf("reference.txt has %d results, cases have %d", len(ref.Results), len(cases))
	}

	for i, c := range cases {
		if got := ref.Results[i].ID; got != c.ID {
			t.Fatalf("result %d is %s, want %s", i, got, c.ID)
		}
	}
}

// TestCases requires every case ID to be unique, free of white space and to
// end in its image's size, and every image to be within format.MaxSize on each
// side with four straight bytes a pixel.
func TestCases(t *testing.T) {
	seen := map[string]bool{}

	for _, c := range Cases() {
		if seen[c.ID] {
			t.Errorf("duplicate ID %s", c.ID)
		}

		seen[c.ID] = true

		if strings.ContainsAny(c.ID, " \t\n") {
			t.Errorf("ID %q contains white space", c.ID)
		}

		img := c.Image()
		s := img.Bounds().Size()

		if s.X < 1 || s.Y < 1 || s.X > format.MaxSize || s.Y > format.MaxSize {
			t.Errorf("%s: %v is outside 1x1 to 100x100", c.ID, s)
		}

		if !strings.HasSuffix(c.ID, "/"+size(s)) {
			t.Errorf("%s: image is %s", c.ID, size(s))
		}

		if got := len(Straight(img)); got != 4*s.X*s.Y {
			t.Errorf("%s: %d straight bytes, want %d", c.ID, got, 4*s.X*s.Y)
		}
	}
}

// TestTies requires each tie case to round an exact half with an even integer
// part when computing a luminance component count, one of the first two
// unrounded values.
func TestTies(t *testing.T) {
	ref, err := Reference()
	if err != nil {
		t.Fatal(err)
	}

	n := 0

	for _, r := range ref.Results {
		if !strings.HasPrefix(r.ID, "tie/") {
			continue
		}

		n++

		if !evenHalf(r.Unrounded[0]) && !evenHalf(r.Unrounded[1]) {
			t.Errorf("%s: component count inputs %v and %v, want one exactly on an even half", r.ID, r.Unrounded[0], r.Unrounded[1])
		}
	}

	if n != 6 {
		t.Errorf("%d tie cases, want 6", n)
	}
}

// TestUnroundedFitHash requires each hash to be as long as its unrounded
// values make it. The reference rounds the two luminance component counts,
// six header fields, two more with alpha, then one value per AC coefficient,
// packed two to a byte after the header.
func TestUnroundedFitHash(t *testing.T) {
	ref, err := Reference()
	if err != nil {
		t.Fatal(err)
	}

	for _, r := range ref.Results {
		if len(r.Hash) < format.MinLen {
			t.Errorf("%s: hash is %d bytes", r.ID, len(r.Hash))

			continue
		}

		h := format.Parse(r.Hash)

		fixed := 8
		if h.Alpha {
			fixed = 10
		}

		if got := len(r.Unrounded) - fixed; got != h.ACCount() || len(r.Hash) != h.HashLen() {
			t.Errorf("%s: %d unrounded values after the fixed %d, header wants %d; hash %d bytes, header wants %d", r.ID, got, fixed, h.ACCount(), len(r.Hash), h.HashLen())
		}
	}
}

// TestDecodeMatchesCases requires decode.txt to be the reference's output for
// today's decode cases: the same fingerprint, count, IDs and hashes in order.
func TestDecodeMatchesCases(t *testing.T) {
	ref, err := Reference()
	if err != nil {
		t.Fatal(err)
	}

	decodeRef, err := DecodeReference()
	if err != nil {
		t.Fatal(err)
	}

	cases := DecodeCases(ref)

	if got, want := DecodeFingerprint(cases), decodeRef.Value("fingerprint"); got != want {
		t.Fatalf("hashes have fingerprint %s, but decode.txt is the reference's output for hashes with fingerprint %s; its header names the reference and the runtime that wrote it", got, want)
	}

	if len(decodeRef.Results) != len(cases) {
		t.Fatalf("decode.txt has %d results, cases have %d", len(decodeRef.Results), len(cases))
	}

	for i, c := range cases {
		if r := decodeRef.Results[i]; r.ID != c.ID || !bytes.Equal(r.Hash, c.Hash) {
			t.Fatalf("result %d is %s %x, want %s %x", i, r.ID, r.Hash, c.ID, c.Hash)
		}
	}
}

func evenHalf(x float64) bool {
	f := math.Floor(x)

	return x-f == 0.5 && math.Mod(f, 2) == 0
}
