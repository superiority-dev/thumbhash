// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package ycbcr converts YCbCr to RGB as color.YCbCr's RGBA method does, in
// arithmetic small enough to inline: the same fixed-point sums, with that
// method's clamping done by table lookup instead of branches.
package ycbcr

// clamp holds i-256 clamped to [0, 255] at i.
var clamp = func() (t [1024]uint8) {
	for i := range t {
		t[i] = uint8(min(max(i-256, 0), 255))
	}

	return t
}()

// RGB returns the 8-bit values color.YCbCr{Y: y, Cb: cb, Cr: cr}.RGBA
// gives, each shifted right by 8. That method clamps each sum to
// [0, 0xffffff] and keeps its top 16 bits, so the top byte is the clamped
// sum shifted right by 16.
func RGB(y, cb, cr uint8) (r, g, b uint8) {
	// The JFIF factors 1.402, 0.34414, 0.71414 and 1.772 in 16.16 fixed
	// point; y times 0x10101 repeats its byte, as 0x12 becomes 0x121212.
	yy1 := int32(y) * 0x10101
	cb1 := int32(cb) - 128
	cr1 := int32(cr) - 128

	return top(yy1 + 91881*cr1), top(yy1 - 22554*cb1 - 46802*cr1), top(yy1 + 116130*cb1)
}

// top returns the top byte of v clamped to [0, 0xffffff], which is v>>16
// clamped to [0, 255], by table lookup. For RGB's sums v>>16 lies in
// [-227, 481], so the index lies in [29, 737].
func top(v int32) uint8 {
	return clamp[(v>>16+256)&1023]
}
