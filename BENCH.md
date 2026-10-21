# Benchmarks

How the speed figures in the [README](README.md) were measured, and every number
behind them.

## What was timed

Encoding and decoding 6 noise images: 32x24, 100x75 and 100x100 pixels, each
opaque and with alpha (transparency). Every implementation got the same 8-bit
RGBA pixels, not premultiplied by alpha. So no image file decoding is
in any number. This package was timed through `Encode` and `Decode`, on its
`*image.NRGBA` path.

Each figure is an implementation's median time over 6 runs, as a multiple of
this package's.

## Machines and versions

Every implementation ran in its own cloud sandboxes, on each architecture:

- arm64: Arm Neoverse V2, 4 vCPU, 8 GiB RAM
- amd64: 4 vCPU, 8 GiB RAM

Each run was limited to 2 of the 4 CPUs.

| implementation | version | how it ran |
|---|---|---|
| this package | this repository | Go 1.27.2 |
| [go.n16f.net/thumbhash](https://pkg.go.dev/go.n16f.net/thumbhash) | v1.1.0 | Go 1.27.2 |
| [evanw/thumbhash](https://github.com/evanw/thumbhash), the original | commit `d2e50a74` | JavaScript, Node 22.23.3 |

## Encoding

This package's time, then every other implementation as a multiple of it.

**arm64**

| image | this package | go.n16f.net | original |
|---|---|---|---|
| 32x24, opaque | 9.49 µs | 5.2x | 6.9x |
| 100x75, opaque | 91.41 µs | 3.7x | 5.8x |
| 100x100, opaque | 130.44 µs | 3.8x | 5.6x |
| 32x24, alpha | 12.36 µs | 4.5x | 5.9x |
| 100x75, alpha | 119.23 µs | 3.2x | 4.9x |
| 100x100, alpha | 160.50 µs | 3.2x | 4.7x |

**amd64**

| image | this package | go.n16f.net | original |
|---|---|---|---|
| 32x24, opaque | 9.92 µs | 6.4x | 6.6x |
| 100x75, opaque | 90.46 µs | 4.9x | 5.6x |
| 100x100, opaque | 149.31 µs | 4.2x | 4.5x |
| 32x24, alpha | 13.62 µs | 5.3x | 5.4x |
| 100x75, alpha | 126.50 µs | 3.9x | 4.4x |
| 100x100, alpha | 169.71 µs | 3.8x | 4.2x |

## Decoding

The hashes of the same 6 images. This package's time, then every other
implementation as a multiple of it.

**arm64**

| image | this package | go.n16f.net | original |
|---|---|---|---|
| 32x24, opaque | 13.74 µs | 9.68x | 8.40x |
| 100x75, opaque | 13.79 µs | 9.61x | 8.36x |
| 100x100, opaque | 19.70 µs | 10.6x | 9.95x |
| 32x24, alpha | 16.56 µs | 8.28x | 7.67x |
| 100x75, alpha | 16.58 µs | 8.26x | 7.55x |
| 100x100, alpha | 20.10 µs | 8.53x | 7.85x |

**amd64**

| image | this package | go.n16f.net | original |
|---|---|---|---|
| 32x24, opaque | 17.84 µs | 9.98x | 8.12x |
| 100x75, opaque | 18.07 µs | 9.85x | 8.05x |
| 100x100, opaque | 25.73 µs | 10.5x | 9.13x |
| 32x24, alpha | 27.08 µs | 6.92x | 5.71x |
| 100x75, alpha | 26.95 µs | 6.94x | 5.77x |
| 100x100, alpha | 32.81 µs | 7.05x | 5.89x |

## Memory

Allocated per call by the Go implementations, the same on both architectures.

| package | encode | decode |
|---|---|---|
| this package | 24 to 32 B, 1 allocation | 3,136 to 4,160 B, 2 allocations |
| this package, `DecodeInto` | | 0 B |
| go.n16f.net | 600 to 688 B, 6 to 8 allocations | 84,128 to 135,976 B, 1,488 to 2,068 allocations |

## Agreement with the original

On the 259 test images, from the same pixels as the timings, how many hashes
each gives exactly as the original does on amd64:

| implementation | arm64 | amd64 |
|---|---|---|
| evanw/thumbhash, the original | 235 | 259 |
| this package | 259 | 259 |

The original gives 24 different hashes on arm64, because its cosine differs
there in the last bit. This package gives the amd64 hashes on both. Decoding
gives the original's images for all 291 test hashes, on both.

## Reference hashes

`internal/testimage/testdata/reference.txt` holds the original's hash of every
test image, and `internal/testimage/testdata/decode.txt` its decoded images of
291 hashes. Both were made by the original at commit `d2e50a74`, under
Node 22.19.0 on amd64.

The programs that made these files and ran the timings are not in this
repository. Open an issue if you would like them published.
