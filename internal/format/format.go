// Package format defines the ThumbHash byte layout: the header fields and how
// many coefficients follow them.
package format

// MaxSize is the largest image width or height.
const MaxSize = 100

// MinLen is the header length in bytes without alpha.
const MinLen = 5

// The component counts the format fixes: luminance has LumLong components on
// its longer side, LumLongAlpha with alpha, and at least MinComponents on
// each; P and Q have PQSide on each side, and A has ASide.
const (
	LumLong       = 7
	LumLongAlpha  = 5
	MinComponents = 3
	PQSide        = 3
	ASide         = 5
)

// Header is the start of a hash. L is luminance, P and Q are color, A is
// alpha; each channel has an average (DC) and a scale for its coefficients.
type Header struct {
	LDC, PDC, QDC    uint8 // 6 bits each
	LScale           uint8 // 5 bits
	PScale, QScale   uint8 // 6 bits each
	Count            uint8 // 3 bits: L components on the shorter side
	ADC, AScale      uint8 // 4 bits each, alpha only
	Alpha, Landscape bool
}

// Parse reads the header from hash, which must hold at least MinLen bytes. It
// reads the alpha byte only if hash has one.
func Parse(hash []byte) Header {
	h24 := uint32(hash[0]) | uint32(hash[1])<<8 | uint32(hash[2])<<16
	h16 := uint32(hash[3]) | uint32(hash[4])<<8

	h := Header{
		LDC:       uint8(h24 & 63),
		PDC:       uint8(h24 >> 6 & 63),
		QDC:       uint8(h24 >> 12 & 63),
		LScale:    uint8(h24 >> 18 & 31),
		Alpha:     h24>>23 != 0,
		Count:     uint8(h16 & 7),
		PScale:    uint8(h16 >> 3 & 63),
		QScale:    uint8(h16 >> 9 & 63),
		Landscape: h16>>15 != 0,
	}

	if h.Alpha && len(hash) > MinLen {
		h.ADC, h.AScale = hash[5]&15, hash[5]>>4
	}

	return h
}

// Append packs h onto the end of dst.
func (h Header) Append(dst []byte) []byte {
	h24 := uint32(h.LDC) | uint32(h.PDC)<<6 | uint32(h.QDC)<<12 | uint32(h.LScale)<<18 | bit(h.Alpha)<<23
	h16 := uint32(h.Count) | uint32(h.PScale)<<3 | uint32(h.QScale)<<9 | bit(h.Landscape)<<15
	dst = append(dst, byte(h24), byte(h24>>8), byte(h24>>16), byte(h16), byte(h16>>8))

	if h.Alpha {
		dst = append(dst, h.ADC|h.AScale<<4)
	}

	return dst
}

// Size returns the header length: MinLen bytes, plus one with alpha.
func (h Header) Size() int {
	if h.Alpha {
		return MinLen + 1
	}

	return MinLen
}

// HashLen returns the full hash length: the header, then 4 bits per
// coefficient.
func (h Header) HashLen() int {
	return h.Size() + (h.ACCount()+1)/2
}

// ACCount returns how many coefficients follow the header: L's, then P's and
// Q's, then A's if there is alpha.
func (h Header) ACCount() int {
	lx, ly := h.Components()

	n := acCount(lx, ly) + 2*acCount(PQSide, PQSide)
	if h.Alpha {
		n += acCount(ASide, ASide)
	}

	return n
}

// Components returns L's component counts across and down: Count on the
// shorter side, LumLong or LumLongAlpha on the longer, each at least
// MinComponents.
func (h Header) Components() (lx, ly int) {
	long := LumLong
	if h.Alpha {
		long = LumLongAlpha
	}

	if h.Landscape {
		return max(MinComponents, long), max(MinComponents, int(h.Count))
	}

	return max(MinComponents, int(h.Count)), max(MinComponents, long)
}

// AspectRatio returns width over height as the header stores it: Count over
// LumLong or LumLongAlpha, inverted for landscape.
func (h Header) AspectRatio() float64 {
	count := float64(h.Count)

	long := float64(LumLong)
	if h.Alpha {
		long = LumLongAlpha
	}

	if h.Landscape {
		return long / count
	}

	return count / long
}

// acCount returns how many coefficients a channel with nx by ny components
// stores, not counting its average.
func acCount(nx, ny int) int {
	n := -1
	for cy := range ny {
		for cx := 0; cx*ny < nx*(ny-cy); cx++ {
			n++
		}
	}

	return n
}

func bit(b bool) uint32 {
	if b {
		return 1
	}

	return 0
}
