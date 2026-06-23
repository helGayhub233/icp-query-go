package captcha

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"sort"
	"strings"

	_ "image/png" // register PNG decoder for image.Decode
)

// MatchSliderOffset finds the x-offset of the gap in a slider captcha.
// smallImgB64 and bigImgB64 are base64-encoded images (PNG or JPEG).
// Returns (true, offset_x) on success, (false, 0) if no gap found.
//
// Algorithm:
//  1. Downsample big image by 2x
//  2. Quantize colors (RGB/4*4)
//  3. Encode RGB as single uint32 per pixel
//  4. Find top 5 most frequent colors
//  5. Column run-length encoding to find rectangular gap regions
//  6. Return x-offset of the largest near-square region
func MatchSliderOffset(smallImgB64, bigImgB64 string) (bool, int, error) {
	smallImg, err := decodeBase64Image(smallImgB64)
	if err != nil {
		return false, 0, fmt.Errorf("decode small image: %w", err)
	}

	bigImg, err := decodeBase64Image(bigImgB64)
	if err != nil {
		return false, 0, fmt.Errorf("decode big image: %w", err)
	}

	smallBounds := smallImg.Bounds()
	sw := smallBounds.Dx()
	sh := smallBounds.Dy()

	// Downsample big image by 2x
	resized := downsample2x(bigImg)
	h := resized.Bounds().Dy()
	w := resized.Bounds().Dx()

	minSide := int(float64(min(sw, sh)) * 0.5 * 0.5)
	if minSide < 1 {
		minSide = 1
	}

	// Quantize colors and encode as single uint32
	colorID := make([][]uint32, h)
	for y := 0; y < h; y++ {
		colorID[y] = make([]uint32, w)
		for x := 0; x < w; x++ {
			r, g, b, _ := resized.At(x, y).RGBA()
			// RGBA() returns 0-65535, scale to 0-255
			r8 := uint32(r >> 8)
			g8 := uint32(g >> 8)
			b8 := uint32(b >> 8)
			qr := (r8 / 4) * 4
			qg := (g8 / 4) * 4
			qb := (b8 / 4) * 4
			colorID[y][x] = qr + qg*256 + qb*65536
		}
	}

	// Count color frequencies
	colorCount := make(map[uint32]int)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			colorCount[colorID[y][x]]++
		}
	}

	// Get top 5 colors by frequency
	topColors := topNColors(colorCount, 5)

	bestArea := 0
	bestX := 0

	for _, c := range topColors {
		// Build mask for this color
		mask := make([][]bool, h)
		for y := 0; y < h; y++ {
			mask[y] = make([]bool, w)
			for x := 0; x < w; x++ {
				mask[y][x] = colorID[y][x] == c
			}
		}

		// Column run-length encoding: col_run[y][x] = consecutive true count upward
		colRun := make([][]int32, h)
		for y := 0; y < h; y++ {
			colRun[y] = make([]int32, w)
		}
		for x := 0; x < w; x++ {
			if mask[0][x] {
				colRun[0][x] = 1
			}
		}
		for y := 1; y < h; y++ {
			for x := 0; x < w; x++ {
				if mask[y][x] {
					colRun[y][x] = colRun[y-1][x] + 1
				}
			}
		}

		// Scan rows for contiguous segments with sufficient height
		for y := minSide; y < h; y++ {
			// Find contiguous segments where colRun >= minSide
			row := make([]bool, w)
			for x := 0; x < w; x++ {
				row[x] = colRun[y][x] >= int32(minSide)
			}

			// Skip if no matching columns
			hasAny := false
			for x := 0; x < w; x++ {
				if row[x] {
					hasAny = true
					break
				}
			}
			if !hasAny {
				continue
			}

			// Find runs of true values
			segments := findTrueRuns(row)
			for _, seg := range segments {
				runW := seg.end - seg.start
				// Skip if too far left (the slider piece itself)
				if seg.start <= sw/4 {
					continue
				}
				runH := int(colRun[y][seg.start])
				if runH == 0 {
					continue
				}
				ratio := float64(runW) / float64(runH)
				if ratio > 0.7 && ratio < 1.4 {
					area := runW * runH
					if area > bestArea {
						bestArea = area
						bestX = seg.start
					}
				}
			}
		}
	}

	if bestArea == 0 {
		return false, 0, nil
	}

	offsetX := bestX * 2 // Scale back up
	return true, offsetX, nil
}

// decodeBase64Image decodes a base64 string into an image.Image.
// Tries generic decode first (PNG is the primary format from MIIT),
// falls back to JPEG-only decoder if the format is unrecognized.
func decodeBase64Image(b64 string) (image.Image, error) {
	// Strip data URI prefix if present (e.g. "data:image/png;base64,")
	if idx := strings.Index(b64, ";base64,"); idx != -1 {
		b64 = b64[idx+8:]
	}

	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}

	// Try generic decode first — MIIT returns PNG images
	img, _, err := image.Decode(bytes.NewReader(data))
	if err == nil {
		return img, nil
	}

	// Fallback to JPEG-only decoder
	img, err = jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("image decode: %w", err)
	}
	return img, nil
}

// downsample2x reduces image dimensions by 2x.
func downsample2x(img image.Image) *image.NRGBA {
	srcBounds := img.Bounds()
	srcW := srcBounds.Dx()
	srcH := srcBounds.Dy()

	dstW := srcW / 2
	dstH := srcH / 2

	dst := image.NewNRGBA(image.Rect(0, 0, dstW, dstH))
	for y := 0; y < dstH; y++ {
		for x := 0; x < dstW; x++ {
			dst.Set(x, y, img.At(x*2, y*2))
		}
	}
	return dst
}

// topNColors returns the top N colors by frequency.
func topNColors(counts map[uint32]int, n int) []uint32 {
	type kv struct {
		color uint32
		count int
	}
	var entries []kv
	for k, v := range counts {
		entries = append(entries, kv{k, v})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].count > entries[j].count
	})
	if len(entries) > n {
		entries = entries[:n]
	}
	result := make([]uint32, len(entries))
	for i, e := range entries {
		result[i] = e.color
	}
	return result
}

type segment struct {
	start int
	end   int
}

// findTrueRuns finds contiguous runs of true values in a bool slice.
func findTrueRuns(row []bool) []segment {
	var segs []segment
	inRun := false
	start := 0
	for i, v := range row {
		if v && !inRun {
			start = i
			inRun = true
		} else if !v && inRun {
			segs = append(segs, segment{start, i})
			inRun = false
		}
	}
	if inRun {
		segs = append(segs, segment{start, len(row)})
	}
	return segs
}
