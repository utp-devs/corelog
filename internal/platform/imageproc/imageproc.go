// Package imageproc validates uploaded images and turns them into square PNG thumbnails.
// The content type is decided by sniffing the bytes, never by the client-declared type.
package imageproc

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
	"net/http"

	"image/jpeg"

	"golang.org/x/image/draw"
	"golang.org/x/image/webp"
)

const (
	// MaxUploadBytes is the largest accepted upload.
	MaxUploadBytes = 2 << 20
	// MaxDimension is the largest accepted width and height, checked before any pixel is decoded.
	MaxDimension = 4096
	// OutputSize is the side of the square PNG produced by Process.
	OutputSize = 256
)

var (
	// ErrTooLarge is returned when the data exceeds MaxUploadBytes.
	ErrTooLarge = errors.New("imageproc: image too large")
	// ErrUnsupportedType is returned when the sniffed type is not PNG, JPEG or WebP.
	ErrUnsupportedType = errors.New("imageproc: unsupported image type")
	// ErrInvalid is returned for corrupt, truncated or oversized-dimension images.
	ErrInvalid = errors.New("imageproc: invalid image")
)

// Process validates data and returns a OutputSize x OutputSize PNG: the image is center-cropped to a
// square and resized. Process never trusts declared metadata; it sniffs the type and reads the header
// dimensions before decoding the pixels, so a decompression bomb is rejected cheaply.
func Process(data []byte) ([]byte, error) {
	if len(data) > MaxUploadBytes {
		return nil, ErrTooLarge
	}
	if len(data) == 0 {
		return nil, ErrInvalid
	}
	sniff := data
	if len(sniff) > 512 {
		sniff = sniff[:512]
	}
	kind := http.DetectContentType(sniff)
	switch kind {
	case "image/png", "image/jpeg", "image/webp":
	default:
		return nil, ErrUnsupportedType
	}

	cfg, err := decodeConfig(kind, data)
	if err != nil {
		return nil, ErrInvalid
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > MaxDimension || cfg.Height > MaxDimension {
		return nil, fmt.Errorf("%w: dimensions %dx%d exceed %dx%d", ErrInvalid, cfg.Width, cfg.Height, MaxDimension, MaxDimension)
	}

	src, err := decode(kind, data)
	if err != nil {
		return nil, ErrInvalid
	}

	out := cropSquareResize(src)
	var buf bytes.Buffer
	if err := png.Encode(&buf, out); err != nil {
		return nil, fmt.Errorf("imageproc: encode png: %w", err)
	}
	return buf.Bytes(), nil
}

func decodeConfig(kind string, data []byte) (cfg image.Config, err error) {
	defer recoverInvalid(&err)
	r := bytes.NewReader(data)
	switch kind {
	case "image/png":
		return png.DecodeConfig(r)
	case "image/jpeg":
		return jpeg.DecodeConfig(r)
	default:
		return webp.DecodeConfig(r)
	}
}

func decode(kind string, data []byte) (img image.Image, err error) {
	defer recoverInvalid(&err)
	r := bytes.NewReader(data)
	switch kind {
	case "image/png":
		return png.Decode(r)
	case "image/jpeg":
		return jpeg.Decode(r)
	default:
		return webp.Decode(r)
	}
}

// recoverInvalid turns a decoder panic on malformed input into ErrInvalid.
func recoverInvalid(err *error) {
	if r := recover(); r != nil {
		*err = fmt.Errorf("%w: decoder panic: %v", ErrInvalid, r)
	}
}

func cropSquareResize(src image.Image) *image.RGBA {
	b := src.Bounds()
	side := min(b.Dx(), b.Dy())
	x0 := b.Min.X + (b.Dx()-side)/2
	y0 := b.Min.Y + (b.Dy()-side)/2
	dst := image.NewRGBA(image.Rect(0, 0, OutputSize, OutputSize))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, image.Rect(x0, y0, x0+side, y0+side), draw.Src, nil)
	return dst
}
