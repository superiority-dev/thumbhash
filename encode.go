package thumbhash

import (
	"fmt"
	"image"

	"github.com/superiority-dev/thumbhash/internal/codec"
)

// An Encoder computes ThumbHashes and reuses its memory between calls. The
// zero value is ready to use. An Encoder should be used by one goroutine at a
// time; [Encode] is safe for concurrent use.
type Encoder struct {
	c codec.Encoder
}

// Encode returns the ThumbHash of img, as [Encode] does.
func (e *Encoder) Encode(img image.Image) ([]byte, error) {
	b := img.Bounds()
	if b.Empty() {
		return nil, fmt.Errorf("%w: %v", ErrEmptyImage, b)
	}

	// A side past the int range wraps negative in Size, and is exact as a uint.
	s := b.Size()
	if uint(s.X) > MaxSize || uint(s.Y) > MaxSize {
		return nil, fmt.Errorf("%w: %dx%d, need at most %dx%d", ErrImageTooLarge, uint(s.X), uint(s.Y), MaxSize, MaxSize)
	}

	e.c.Pix = codec.Straight(e.c.Pix[:0], img)
	vals, hasAlpha := e.c.Unrounded(e.c.Pix, s.X, s.Y)

	return codec.Quantize(vals, s.X, s.Y, hasAlpha), nil
}

// Encode returns the ThumbHash of img, or [ErrEmptyImage] or
// [ErrImageTooLarge]. Each pixel is read as the straight, not premultiplied,
// RGBA that [color.NRGBAModel] converts it to. Encode is safe for concurrent
// use.
func Encode(img image.Image) ([]byte, error) {
	var e Encoder

	return e.Encode(img)
}
