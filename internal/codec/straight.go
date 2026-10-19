package codec

import (
	"image"
	"image/color"
)

// Straight appends img's pixels to dst in row order, each as the 8-bit
// straight RGBA that [color.NRGBAModel] converts it to.
func Straight(dst []byte, img image.Image) []byte {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			dst = append(dst, c.R, c.G, c.B, c.A)
		}
	}

	return dst
}
