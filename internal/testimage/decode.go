package testimage

import (
	"crypto/sha256"
	_ "embed"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/superiority-dev/thumbhash/internal/format"
)

//go:embed testdata/decode.txt
var decoded string

// DecodeCase is one hash the decoder tests run on.
type DecodeCase struct {
	ID   string
	Hash []byte
}

// DecodeResult is what the reference decoder produced for one hash.
type DecodeResult struct {
	ID   string
	Hash []byte
	W, H int

	// Ratio is the aspect ratio the reference returns for Hash.
	Ratio float64

	// RGBA is the SHA-256 of the decoded bytes, Values that of the value
	// each byte was truncated from, as big-endian float64 bits. The values
	// are those of the runtime the file names; another runtime may differ.
	RGBA, Values string
}

// MatchesRGBA reports whether the decoded bytes pix have the SHA-256 in RGBA.
func (r DecodeResult) MatchesRGBA(pix []byte) bool {
	sum := sha256.Sum256(pix)

	return hex.EncodeToString(sum[:]) == r.RGBA
}

// MatchesValues reports whether vals, the values the decoded bytes are
// truncated from, have the SHA-256 in Values.
func (r DecodeResult) MatchesValues(vals []float64) bool {
	// binary.Write would return an error a hash never gives.
	h := sha256.New()
	for _, v := range vals {
		h.Write(binary.BigEndian.AppendUint64(nil, math.Float64bits(v)))
	}

	return hex.EncodeToString(h.Sum(nil)) == r.Values
}

// DecodeExpected is a parsed decode file.
type DecodeExpected struct {
	header  []string
	Results []DecodeResult
}

// DecodeReference parses testdata/decode.txt: header lines start with "# ",
// every other line is
//
//	<case id> <hash> <w> <h> <ratio> <sha256 of RGBA> <sha256 of values>
func DecodeReference() (DecodeExpected, error) {
	header, results, err := parseFile(decoded, parseDecodeResult)
	if err != nil {
		return DecodeExpected{}, err
	}

	return DecodeExpected{header: header, Results: results}, nil
}

// Value returns the rest of the header line starting with key, or "" if
// there is none.
func (e DecodeExpected) Value(key string) string {
	return Expected{header: e.header}.Value(key)
}

// DecodeCases returns the hashes in e under their case IDs, followed by
// generated hashes covering every combination of the header's luminance
// component count, orientation and alpha. The encoder never writes eight of
// them: count 0, and counts 6 and 7 with alpha.
func DecodeCases(e Expected) []DecodeCase {
	out := make([]DecodeCase, 0, len(e.Results))
	for _, r := range e.Results {
		out = append(out, DecodeCase{ID: r.ID, Hash: r.Hash})
	}

	return append(out, generated()...)
}

// DecodeFingerprint returns the SHA-256 of every case ID and hash.
func DecodeFingerprint(cases []DecodeCase) string {
	h := sha256.New()
	for _, c := range cases {
		fmt.Fprintf(h, "%s %x\n", c.ID, c.Hash)
	}

	return hex.EncodeToString(h.Sum(nil))
}

func generated() []DecodeCase {
	r := newSplitMix(headerSeed)
	nibble := func() uint8 { return r.byte() & 15 }

	var out []DecodeCase

	// Every luminance component count, 0 to 7, in both orientations, with
	// and without alpha.
	for _, alpha := range []string{"opaque", "alpha"} {
		for _, side := range []string{"portrait", "landscape"} {
			for count := range uint8(8) {
				h := randomHeader(r)
				h.Alpha, h.Landscape, h.Count = alpha == "alpha", side == "landscape", count
				out = append(out, DecodeCase{
					ID:   fmt.Sprintf("gen/header/%s-%s-%d", alpha, side, count),
					Hash: generatedHash(h, nibble),
				})
			}
		}
	}

	return out
}

// randomHeader draws every header field from r, each within its bit width.
func randomHeader(r *splitMix) format.Header {
	return format.Header{
		LDC: r.byte() & 63, PDC: r.byte() & 63, QDC: r.byte() & 63,
		LScale: r.byte() & 31, PScale: r.byte() & 63, QScale: r.byte() & 63,
		Count: r.byte() & 7, ADC: r.byte() & 15, AScale: r.byte() & 15,
		Alpha: r.byte()&1 == 1, Landscape: r.byte()&1 == 1,
	}
}

// generatedHash packs h and as many AC values from nibble as h says follow it.
func generatedHash(h format.Header, nibble func() uint8) []byte {
	out := h.Append(nil)

	for i, n := 0, h.ACCount(); i < n; i += 2 {
		b := nibble()
		if i+1 < n {
			b |= nibble() << 4
		}

		out = append(out, b)
	}

	return out
}

func parseDecodeResult(line string) (DecodeResult, error) {
	f := strings.Fields(line)
	if len(f) != 7 {
		return DecodeResult{}, fmt.Errorf("%d fields, want 7", len(f))
	}

	r := DecodeResult{ID: f[0], RGBA: f[5], Values: f[6]}

	var err error
	if r.Hash, err = hex.DecodeString(f[1]); err != nil {
		return DecodeResult{}, err
	}

	if r.W, err = strconv.Atoi(f[2]); err != nil {
		return DecodeResult{}, err
	}

	if r.H, err = strconv.Atoi(f[3]); err != nil {
		return DecodeResult{}, err
	}

	if r.Ratio, err = strconv.ParseFloat(f[4], 64); err != nil {
		return DecodeResult{}, err
	}

	return r, nil
}
