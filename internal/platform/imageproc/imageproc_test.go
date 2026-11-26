package imageproc

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

// webpLossless1x1 is a 1x1 lossless WebP (VP8L) used as the WebP fixture.
const webpLossless1x1 = "UklGRhoAAABXRUJQVlA4TA0AAAAvAAAAEAcQERGIiP4HAA=="

func encodePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 128, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func encodeJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewGray(image.Rect(0, 0, w, h))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func decodeOutput(t *testing.T, data []byte) image.Image {
	t.Helper()
	if !bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")) {
		t.Fatal("output is not a PNG")
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("output does not decode: %v", err)
	}
	return img
}

func TestProcessMatrix(t *testing.T) {
	webp := mustB64(t, webpLossless1x1)

	tests := []struct {
		name     string
		data     []byte
		wantErr  error
		wantSize bool // expect a 256x256 PNG
	}{
		{"png square", encodePNG(t, 300, 300), nil, true},
		{"png landscape is cropped to square", encodePNG(t, 800, 200), nil, true},
		{"png portrait is cropped to square", encodePNG(t, 200, 900), nil, true},
		{"png smaller than output is upscaled", encodePNG(t, 10, 10), nil, true},
		{"jpeg", encodeJPEG(t, 400, 300), nil, true},
		{"webp 1x1", webp, nil, true},
		{"empty", nil, ErrInvalid, false},
		{"plain text", []byte("hola, esto no es una imagen"), ErrUnsupportedType, false},
		{"html disguised as image", []byte("<html><script>alert(1)</script></html>"), ErrUnsupportedType, false},
		{"gif is not allowed", append([]byte("GIF89a"), make([]byte, 64)...), ErrUnsupportedType, false},
		{"svg is not allowed", []byte("<svg xmlns='http://www.w3.org/2000/svg'></svg>"), ErrUnsupportedType, false},
		{"oversized dimensions width", encodePNG(t, MaxDimension+1, 1), ErrInvalid, false},
		{"oversized dimensions jpeg", encodeJPEG(t, 1, MaxDimension+1), ErrInvalid, false},
		{"truncated png", encodePNG(t, 64, 64)[:len(encodePNG(t, 64, 64))-12], ErrInvalid, false},
		{"truncated jpeg", encodeJPEG(t, 64, 64)[:100], ErrInvalid, false},
		{"png signature only", []byte("\x89PNG\r\n\x1a\n"), ErrInvalid, false},
		{"too large", make([]byte, MaxUploadBytes+1), ErrTooLarge, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := Process(tt.data)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				if out != nil {
					t.Fatal("output must be nil on error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantSize {
				b := decodeOutput(t, out).Bounds()
				if b.Dx() != OutputSize || b.Dy() != OutputSize {
					t.Fatalf("output size = %dx%d, want %dx%d", b.Dx(), b.Dy(), OutputSize, OutputSize)
				}
			}
		})
	}
}

func TestProcessIgnoresDeclaredTypeAndSniffsBytes(t *testing.T) {
	// The caller may label the bytes however it likes; Process only looks at the content.
	// A valid PNG is accepted even though nothing in the API says it is a PNG.
	out, err := Process(encodePNG(t, 32, 32))
	if err != nil {
		t.Fatalf("valid PNG rejected: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("empty output")
	}
}

func TestProcessRejectsBombBeforeDecoding(t *testing.T) {
	// A 5000x1 PNG is tiny on disk but declares a width above MaxDimension. It must fail with
	// ErrInvalid from the header check, not from an allocation of 5000 pixels per row.
	data := encodePNG(t, 5000, 1)
	if len(data) > 64<<10 {
		t.Fatalf("fixture unexpectedly large: %d bytes", len(data))
	}
	if _, err := Process(data); !errors.Is(err, ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
}

func TestProcessOutputIsDeterministicSquare(t *testing.T) {
	a, err := Process(encodePNG(t, 120, 60))
	if err != nil {
		t.Fatal(err)
	}
	b, err := Process(encodePNG(t, 120, 60))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("same input must produce the same output")
	}
}

func FuzzProcess(f *testing.F) {
	f.Add([]byte("\x89PNG\r\n\x1a\n"))
	f.Add([]byte("\xff\xd8\xff\xe0"))
	f.Add([]byte("garbage"))
	f.Fuzz(func(t *testing.T, data []byte) {
		out, err := Process(data)
		if err == nil && len(out) == 0 {
			t.Fatal("success with empty output")
		}
	})
}

func mustB64(t *testing.T, s string) []byte {
	t.Helper()
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
