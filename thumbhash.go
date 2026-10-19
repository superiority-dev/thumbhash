// Package thumbhash encodes images as ThumbHashes and decodes them into
// placeholder images. The format is defined at
// https://github.com/evanw/thumbhash.
package thumbhash

import (
	"errors"

	"github.com/superiority-dev/thumbhash/internal/codec"
	"github.com/superiority-dev/thumbhash/internal/format"
)

// MaxSize is 100, the largest width or height Encode accepts.
const MaxSize = format.MaxSize

// DecodedSize is 32, the longest side of an image Decode returns.
const DecodedSize = codec.DecodedSize

var (
	// ErrEmptyImage is returned for an image with no pixels, such as one 0 wide
	// and 500 tall, which is empty rather than too large.
	ErrEmptyImage = errors.New("thumbhash: empty image")

	// ErrImageTooLarge is returned for an image wider or taller than [MaxSize].
	ErrImageTooLarge = errors.New("thumbhash: image too large")

	// ErrInvalidHash is returned for a hash shorter than five bytes, or than its
	// header says it is.
	ErrInvalidHash = errors.New("thumbhash: invalid hash")
)
