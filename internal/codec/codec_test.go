package codec

import "math"

// firstDiff returns the index of the first value of got that differs from
// want's bit for bit, or -1: slices.Equal gives neither the bits nor the index.
func firstDiff(got, want []float64) int {
	for i := range got {
		if math.Float64bits(got[i]) != math.Float64bits(want[i]) {
			return i
		}
	}

	return -1
}
