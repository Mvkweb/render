package imaging

import (
	"image"
	"image/color"

	"github.com/nfnt/resize"
)

// DHash calculates the difference hash of an image.
func DHash(img image.Image) uint64 {
	// 1. Resize the image to 9x8.
	resized := resize.Resize(9, 8, img, resize.Lanczos3)

	// 2. Convert the image to grayscale.
	gray := image.NewGray(resized.Bounds())
	for y := 0; y < resized.Bounds().Dy(); y++ {
		for x := 0; x < resized.Bounds().Dx(); x++ {
			gray.Set(x, y, resized.At(x, y))
		}
	}

	// 3. Calculate the difference.
	var hash uint64
	for i := 0; i < 8; i++ {
		for j := 0; j < 8; j++ {
			if gray.GrayAt(j, i).Y > gray.GrayAt(j+1, i).Y {
				hash |= 1
			}
			hash <<= 1
		}
	}

	return hash
}

// Grayscale converts an image to grayscale.
func Grayscale(img image.Image) image.Image {
	bounds := img.Bounds()
	gray := image.NewGray(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			gray.Set(x, y, color.GrayModel.Convert(img.At(x, y)))
		}
	}
	return gray
}
