package captcha

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

func TestMatchSliderOffset_Synthetic(t *testing.T) {
	// Create a synthetic big image with a gray gap at x=60
	bigImg := createTestImageWithGap(300, 150, 60, 40, 40)
	smallImg := createSolidImage(40, 40, color.Gray{Y: 128})

	bigB64 := encodeImageToBase64(bigImg)
	smallB64 := encodeImageToBase64(smallImg)

	ok, offsetX, err := MatchSliderOffset(smallB64, bigB64)
	if err != nil {
		t.Fatalf("MatchSliderOffset error: %v", err)
	}
	if !ok {
		t.Fatal("MatchSliderOffset failed to find gap")
	}

	// The offset should be near 60 (with some tolerance for downsampling)
	t.Logf("Found gap at x=%d (expected ~60)", offsetX)
	if offsetX < 40 || offsetX > 80 {
		t.Errorf("offset x=%d out of expected range [40, 80]", offsetX)
	}
}

func TestMatchSliderOffset_NoGap(t *testing.T) {
	// Uniform image, no gap
	bigImg := createSolidImage(300, 150, color.NRGBA{R: 200, G: 200, B: 200, A: 255})
	smallImg := createSolidImage(40, 40, color.Gray{Y: 128})

	bigB64 := encodeImageToBase64(bigImg)
	smallB64 := encodeImageToBase64(smallImg)

	ok, _, err := MatchSliderOffset(smallB64, bigB64)
	if err != nil {
		t.Fatalf("MatchSliderOffset error: %v", err)
	}
	if ok {
		t.Error("expected no gap found in uniform image")
	}
}

// createTestImageWithGap creates a white image with a gray rectangular gap.
func createTestImageWithGap(w, h, gapX, gapW, gapH int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))

	// Fill with white
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}

	// Draw gray gap
	for y := h/2 - gapH/2; y < h/2+gapH/2; y++ {
		for x := gapX; x < gapX+gapW; x++ {
			if x >= 0 && x < w && y >= 0 && y < h {
				img.Set(x, y, color.NRGBA{R: 128, G: 128, B: 128, A: 255})
			}
		}
	}

	return img
}

func createSolidImage(w, h int, c color.Color) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

func encodeImageToBase64(img image.Image) string {
	var buf bytes.Buffer
	jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90})
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}
