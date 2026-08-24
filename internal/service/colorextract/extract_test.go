package colorextract

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"strings"
	"testing"
)

func TestExtractAccent_DominantBucket(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			if (x+y)%10 < 7 {
				img.Set(x, y, color.RGBA{0xd4, 0xad, 0x7a, 0xff})
			} else {
				img.Set(x, y, color.RGBA{0x10, 0x10, 0x10, 0xff})
			}
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}

	hex, err := ExtractAccent(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hex, "#") || len(hex) != 7 {
		t.Fatalf("expected #RRGGBB, got %q", hex)
	}
	r := parseHexByte(hex[1:3])
	g := parseHexByte(hex[3:5])
	b := parseHexByte(hex[5:7])
	if r <= g || g <= b {
		t.Fatalf("expected warm tan family R>G>B, got %s", hex)
	}
}

func TestExtractAccent_AllBlackReturnsEmpty(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 50, 50))
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, img, nil)
	hex, err := ExtractAccent(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if hex != "" {
		t.Fatalf("expected empty for all-black, got %q", hex)
	}
}

func TestExtractAccent_BadBytesReturnsError(t *testing.T) {
	_, err := ExtractAccent([]byte("not an image"))
	if err == nil {
		t.Fatal("expected decode error")
	}
}

func parseHexByte(s string) byte {
	var v byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		v <<= 4
		switch {
		case c >= '0' && c <= '9':
			v |= c - '0'
		case c >= 'a' && c <= 'f':
			v |= c - 'a' + 10
		}
	}
	return v
}
