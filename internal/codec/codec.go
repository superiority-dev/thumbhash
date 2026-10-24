// Package codec computes ThumbHashes and decodes them for package thumbhash.
// Each product is rounded before it is added and each sum runs in a fixed
// order, so every value is the same bits on every target.
package codec

// allOpaque reports whether every pixel in the RGBA bytes pix has alpha 255.
func allOpaque(pix []byte) bool {
	for i := 3; i < len(pix); i += 4 {
		if pix[i] != 0xff {
			return false
		}
	}

	return true
}
