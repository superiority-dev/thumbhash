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
| 32x24, opaque | 9.47 µs | 5.2x | 6.9x |
| 100x75, opaque | 91.40 µs | 3.7x | 5.7x |
| 100x100, opaque | 130.29 µs | 3.8x | 5.6x |
| 32x24, alpha | 12.34 µs | 4.6x | 5.9x |
| 100x75, alpha | 118.75 µs | 3.2x | 4.9x |
| 100x100, alpha | 160.39 µs | 3.2x | 4.7x |

**amd64**

| image | this package | go.n16f.net | original |
|---|---|---|---|
| 32x24, opaque | 10.25 µs | 6.3x | 6.2x |
| 100x75, opaque | 93.78 µs | 4.7x | 5.1x |
| 100x100, opaque | 149.41 µs | 4.2x | 4.5x |
| 32x24, alpha | 13.91 µs | 5.2x | 5.2x |
| 100x75, alpha | 126.94 µs | 3.9x | 4.3x |
| 100x100, alpha | 172.49 µs | 3.8x | 4.1x |

## Decoding

The hashes of the same 6 images. This package's time, then every other
implementation as a multiple of it.

**arm64**

| image | this package | go.n16f.net | original |
|---|---|---|---|
| 32x24, opaque | 13.31 µs | 9.86x | 8.64x |
| 100x75, opaque | 13.31 µs | 9.88x | 8.70x |
| 100x100, opaque | 19.00 µs | 10.8x | 10.3x |
| 32x24, alpha | 15.91 µs | 8.55x | 7.82x |
| 100x75, alpha | 15.88 µs | 8.56x | 7.91x |
| 100x100, alpha | 19.26 µs | 8.84x | 8.17x |

**amd64**

| image | this package | go.n16f.net | original |
|---|---|---|---|
| 32x24, opaque | 17.55 µs | 10.2x | 8.22x |
| 100x75, opaque | 17.69 µs | 10.2x | 8.21x |
| 100x100, opaque | 24.91 µs | 10.9x | 9.41x |
| 32x24, alpha | 26.03 µs | 7.36x | 5.91x |
| 100x75, alpha | 25.73 µs | 7.32x | 6.01x |
| 100x100, alpha | 31.71 µs | 7.26x | 6.07x |

## Memory

Allocated per call by the Go implementations, the same on both architectures.

| package | encode | decode |
|---|---|---|
| this package | 24 to 32 B, 1 allocation | 3,136 to 4,160 B, 2 allocations |
| this package, `DecodeInto` | | 0 B |
| go.n16f.net | 600 to 689 B, 6 to 8 allocations | 84,128 to 135,976 B, 1,488 to 2,068 allocations |

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
