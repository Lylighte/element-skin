package homepage

import (
	"bytes"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/webp"
)

var (
	errInvalidImageFormat = errors.New("invalid image format")
	errImageDimensions    = errors.New("image dimensions exceed limit")
)

func validateImageData(data []byte, extension string) error {
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || format != expectedImageFormat(extension) {
		return errInvalidImageFormat
	}
	if config.Width <= 0 || config.Height <= 0 || config.Width > MaxImageDimension || config.Height > MaxImageDimension ||
		int64(config.Width)*int64(config.Height) > MaxImagePixels {
		return errImageDimensions
	}
	return nil
}

func expectedImageFormat(extension string) string {
	switch extension {
	case ".png":
		return "png"
	case ".jpg", ".jpeg":
		return "jpeg"
	case ".webp":
		return "webp"
	default:
		return ""
	}
}
