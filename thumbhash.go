// Package thumbhash encodes images as ThumbHashes and decodes them into
// placeholder images. The format is defined at
// https://github.com/evanw/thumbhash.
package thumbhash

import (
	"errors"

	"github.com/superiority-dev/thumbhash/internal/format"
)

// MaxSize is 100, the largest width or height Encode accepts.
const MaxSize = format.MaxSize

var (
	// ErrEmptyImage is returned for an image with no pixels, such as one 0 wide
	// and 500 tall, which is empty rather than too large.
	ErrEmptyImage = errors.New("thumbhash: empty image")

	// ErrImageTooLarge is returned for an image wider or taller than [MaxSize].
	ErrImageTooLarge = errors.New("thumbhash: image too large")
)
