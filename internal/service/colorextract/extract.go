// Package colorextract derives a usable accent color from a cover image.
//
// Algorithm:
//  1. Decode JPEG/PNG.
//  2. Resize to 32x48 to cheapen histogramming.
//  3. Histogram pixels into 4096 bins (4 bits/channel = 16^3).
//  4. Skip near-black, near-white, and low-saturation pixels.
//  5. Pick the bin with max count whose luminance falls in [0.25, 0.65].
//  6. Return the bin center as #RRGGBB. Empty string = no usable color.
package colorextract

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"

	"golang.org/x/image/draw"
)

// ExtractAccent returns "#RRGGBB" or "" when no usable color was found.
// Returns an error only on decode failure; an unusable image (all dark / all
// desaturated) returns ("", nil) so callers can persist a NULL accent.
func ExtractAccent(imgBytes []byte) (string, error) {
	if len(imgBytes) == 0 {
		return "", errors.New("empty image bytes")
	}
	src, _, err := image.Decode(bytes.NewReader(imgBytes))
	if err != nil {
		return "", fmt.Errorf("decode: %w", err)
	}
	const tw, th = 32, 48
	dst := image.NewRGBA(image.Rect(0, 0, tw, th))
	draw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)

	bins := make(map[uint16]int, 4096)
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			r, g, b, _ := dst.At(x, y).RGBA()
			r8, g8, b8 := byte(r>>8), byte(g>>8), byte(b>>8)
			if int(r8)+int(g8)+int(b8) < 60 {
				continue
			}
			if min3(r8, g8, b8) > 220 {
				continue
			}
			if max3(r8, g8, b8)-min3(r8, g8, b8) < 30 {
				continue
			}
			bins[bin(r8, g8, b8)]++
		}
	}
	if len(bins) == 0 {
		return "", nil
	}

	var bestKey uint16
	bestCount := -1
	for k, c := range bins {
		r8, g8, b8 := unbin(k)
		l := luminance(r8, g8, b8)
		if l < 0.25 || l > 0.65 {
			continue
		}
		if c > bestCount {
			bestCount = c
			bestKey = k
		}
	}
	if bestCount < 0 {
		// No bin in the preferred luminance band — fall back to the absolute
		// dominant color so the title still gets a tint instead of NULL.
		for k, c := range bins {
			if c > bestCount {
				bestCount = c
				bestKey = k
			}
		}
	}
	r8, g8, b8 := unbin(bestKey)
	return fmt.Sprintf("#%02x%02x%02x", r8, g8, b8), nil
}

func bin(r, g, b byte) uint16 {
	return uint16(r>>4)<<8 | uint16(g>>4)<<4 | uint16(b>>4)
}

func unbin(k uint16) (byte, byte, byte) {
	r := byte((k>>8)&0x0F)<<4 | 0x08
	g := byte((k>>4)&0x0F)<<4 | 0x08
	b := byte(k&0x0F)<<4 | 0x08
	return r, g, b
}

func luminance(r, g, b byte) float64 {
	return (0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)) / 255.0
}

func min3(a, b, c byte) byte {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}

func max3(a, b, c byte) byte {
	m := a
	if b > m {
		m = b
	}
	if c > m {
		m = c
	}
	return m
}
