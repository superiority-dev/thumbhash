# thumbhash

[![Go Reference](https://pkg.go.dev/badge/github.com/superiority-dev/thumbhash.svg)](https://pkg.go.dev/github.com/superiority-dev/thumbhash)
[![CI](https://github.com/superiority-dev/thumbhash/actions/workflows/ci.yml/badge.svg)](https://github.com/superiority-dev/thumbhash/actions/workflows/ci.yml)
[![Coverage](https://codecov.io/gh/superiority-dev/thumbhash/graph/badge.svg)](https://codecov.io/gh/superiority-dev/thumbhash)

Fast ThumbHash for Go, with the same result on every machine.

A [ThumbHash](https://evanw.github.io/thumbhash/) is a blurred preview of an
image in at most 25 bytes, shown while the real image loads. This package
encodes and decodes them in pure Go, with no dependencies.

<div align="center">

| <img src=".github/example.png" width="210" alt="An image that this package encodes to IAgGDYIGoodEnoughForAPreview once shrunk"> | &rarr;&nbsp;`IAgGDYIGoodEnoughForAPreview`&nbsp;&rarr; | <img src=".github/example-blurred.png" width="219" alt="IAgGDYIGoodEnoughForAPreview decoded"> |
|:---:|:---:|:---:|
| an image, shrunk to 100x75 | its ThumbHash, in base64 | the hash decoded |

</div>

```
go get github.com/superiority-dev/thumbhash
```

## Why this package

- **Fast.** At least 3.1 times faster than the other Go package and the
  original JavaScript code, at both encoding and decoding.
- **The same hash everywhere.** An image gives the same hash on ARM as on Intel
  or AMD.
- **Exact.** Matches the original JavaScript code's hashes and decoded images,
  bit for bit.

## Speed

Time on noise images from 32x24 to 100x100 pixels, as a multiple of this
package's time.

**Encoding**

| package | language | ARM | Intel or AMD |
|---|---|---|---|
| [go.n16f.net/thumbhash](https://pkg.go.dev/go.n16f.net/thumbhash) | Go | 3.2x to 5.2x | 3.8x to 6.3x |
| [evanw/thumbhash](https://github.com/evanw/thumbhash) | JavaScript | 4.7x to 6.9x | 4.1x to 6.2x |

**Decoding**

| package | language | ARM | Intel or AMD |
|---|---|---|---|
| [go.n16f.net/thumbhash](https://pkg.go.dev/go.n16f.net/thumbhash) | Go | 8.55x to 10.8x | 7.26x to 10.9x |
| [evanw/thumbhash](https://github.com/evanw/thumbhash) | JavaScript | 7.82x to 10.3x | 5.91x to 9.41x |

How it was measured, the exact versions and every number are in
[BENCH.md](BENCH.md).

## Usage

### Encode

```go
hash, err := thumbhash.Encode(img) // img at most 100x100 pixels

var e thumbhash.Encoder // reuses its memory; one goroutine at a time
for _, img := range images {
	hash, err := e.Encode(img)
	// ...
}
```

Fastest with `*image.NRGBA`, `*image.RGBA` and `*image.YCbCr`, which
`image/jpeg` returns. Other image types work too, but slower.

### Decode

```go
img, err := thumbhash.Decode(hash)        // a new image on every call, at most 32x32
ratio, err := thumbhash.AspectRatio(hash) // width over height, without decoding

var wg sync.WaitGroup
for _, batch := range batches {
	wg.Go(func() {
		dst := image.NewNRGBA(image.Rect(0, 0, thumbhash.DecodedSize, thumbhash.DecodedSize)) // reused: DecodeInto allocates nothing
		for _, hash := range batch {
			r, err := thumbhash.DecodeInto(dst, hash) // r: the part of dst it filled
			if err != nil {
				log.Print(err)
				continue
			}
			use(dst, r) // finish with dst, or copy it, before the next decode
		}
	})
}
wg.Wait()
```

## Shrink the image first

A ThumbHash is made from an image at most 100x100 pixels. The original refuses
larger ones, and so does `Encode`. This package does not resize. With
[golang.org/x/image/draw](https://pkg.go.dev/golang.org/x/image/draw):

```go
// Fit inside 100x100, keeping the aspect ratio.
b := img.Bounds()
if b.Empty() {
	return nil, thumbhash.ErrEmptyImage
}
w, h := thumbhash.MaxSize, max(1, thumbhash.MaxSize*b.Dy()/b.Dx())
if b.Dy() > b.Dx() {
	w, h = max(1, thumbhash.MaxSize*b.Dx()/b.Dy()), thumbhash.MaxSize
}

small := image.NewRGBA(image.Rect(0, 0, w, h))
draw.ApproxBiLinear.Scale(small, small.Bounds(), img, b, draw.Src, nil)

hash, err := thumbhash.Encode(small)
```

**Matching hashes made elsewhere.** Resize the same way everywhere. A different
scaler, or a size 1 pixel off, can give a different hash.

## Differences from the original

The original is the JavaScript code in
[evanw/thumbhash](https://github.com/evanw/thumbhash), which defines the format.
This package gives the same hash on every machine. On Intel or AMD, it matches
the original's hash for all 259 test images. The original itself gives a
different hash on ARM for 24 of them.
