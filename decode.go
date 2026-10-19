package thumbhash

import (
	"fmt"
	"image"

	"github.com/superiority-dev/thumbhash/internal/codec"
	"github.com/superiority-dev/thumbhash/internal/format"
)

// Decode returns the image hash describes, at most [DecodedSize] pixels on a
// side, with RGB not premultiplied by alpha, or [ErrInvalidHash]. Decode is
// safe for concurrent use.
func Decode(hash []byte) (*image.NRGBA, error) {
	if err := checkLength(hash); err != nil {
		return nil, err
	}

	var buf [4 * DecodedSize * DecodedSize]float64

	w, h, vals := codec.Render(&buf, hash)

	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i, v := range vals {
		img.Pix[i] = uint8(v)
	}

	return img, nil
}

// AspectRatio returns the width over the height of the image hash was made
// from, approximately: a hash keeps the shorter side over the longer rounded
// to sevenths, or to fifths for an image with alpha, and Encode never stores
// less than 1/7, or 1/5 with alpha. A hash storing 0 gives 0 for a portrait
// and +Inf for a landscape. It returns [ErrInvalidHash] if hash is shorter
// than five bytes.
func AspectRatio(hash []byte) (float64, error) {
	if err := checkHeader(hash); err != nil {
		return 0, err
	}

	return format.Parse(hash).AspectRatio(), nil
}

// checkLength returns an error if hash is shorter than its header requires.
func checkLength(hash []byte) error {
	if err := checkHeader(hash); err != nil {
		return err
	}

	if need := format.Parse(hash).HashLen(); len(hash) < need {
		return fmt.Errorf("%w: %d bytes, need %d as its header says", ErrInvalidHash, len(hash), need)
	}

	return nil
}

// checkHeader returns an error if hash is shorter than the five bytes every
// header has.
func checkHeader(hash []byte) error {
	if len(hash) < format.MinLen {
		return fmt.Errorf("%w: %d bytes, need at least %d", ErrInvalidHash, len(hash), format.MinLen)
	}

	return nil
}
