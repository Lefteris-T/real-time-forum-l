package validation

import (
	"bytes"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/http"

	"forum/internal/model"
)

const (
	MaxPrivateMessageImageSize = 5 << 20
	maxMessageImageDimension   = 4096
	maxMessageImagePixels      = 8_000_000
)

var (
	ErrMessageImageTooLarge = errors.New("message image exceeds the size limit")
	ErrInvalidMessageImage  = errors.New("message image is invalid or unsupported")
)

// ValidatePrivateMessageImage identifies formats from file contents, checks
// the encoded image header, and limits dimensions before persistence.
func ValidatePrivateMessageImage(data []byte) (model.MessageImage, error) {
	if len(data) == 0 {
		return model.MessageImage{}, ErrInvalidMessageImage
	}
	if len(data) > MaxPrivateMessageImageSize {
		return model.MessageImage{}, ErrMessageImageTooLarge
	}
	contentType := http.DetectContentType(data)
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return model.MessageImage{}, ErrInvalidMessageImage
	}
	mimeType := ""
	switch format {
	case "jpeg":
		mimeType = "image/jpeg"
	case "png":
		mimeType = "image/png"
	case "gif":
		mimeType = "image/gif"
	}
	if mimeType == "" || contentType != mimeType {
		return model.MessageImage{}, ErrInvalidMessageImage
	}
	if config.Width < 1 || config.Height < 1 || config.Width > maxMessageImageDimension ||
		config.Height > maxMessageImageDimension || config.Width*config.Height > maxMessageImagePixels {
		return model.MessageImage{}, ErrInvalidMessageImage
	}
	if _, decodedFormat, err := image.Decode(bytes.NewReader(data)); err != nil || decodedFormat != format {
		return model.MessageImage{}, ErrInvalidMessageImage
	}
	return model.MessageImage{MIMEType: mimeType, Data: append([]byte(nil), data...)}, nil
}
