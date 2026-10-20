package format_test

import (
	"bytes"
	"testing"

	"github.com/superiority-dev/thumbhash/internal/format"
	"github.com/superiority-dev/thumbhash/internal/testimage"
)

// TestHeader requires every test hash, encoded or generated, to pack its
// header back to the same bytes and to be HashLen bytes long.
func TestHeader(t *testing.T) {
	decodeRef, err := testimage.DecodeReference()
	if err != nil {
		t.Fatal(err)
	}

	for _, r := range decodeRef.Results {
		h := format.Parse(r.Hash)

		if got := h.Append(nil); !bytes.Equal(got, r.Hash[:h.Size()]) {
			t.Errorf("%s: header %x packs back to %x", r.ID, r.Hash[:h.Size()], got)
		}

		if h.HashLen() != len(r.Hash) {
			t.Errorf("%s: HashLen %d, hash has %d bytes", r.ID, h.HashLen(), len(r.Hash))
		}
	}
}
