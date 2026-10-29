package validation

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"
)

func TestValidatePrivateMessageImageSniffsAndBoundsFormats(t *testing.T) {
	imageData := image.NewRGBA(image.Rect(0, 0, 2, 2))
	imageData.Set(1, 1, color.RGBA{R: 1, G: 2, B: 3, A: 255})
	for _, test := range []struct {
		name   string
		mime   string
		encode func(*bytes.Buffer) error
	}{
		{name: "png", mime: "image/png", encode: func(buffer *bytes.Buffer) error { return png.Encode(buffer, imageData) }},
		{name: "jpeg", mime: "image/jpeg", encode: func(buffer *bytes.Buffer) error { return jpeg.Encode(buffer, imageData, nil) }},
		{name: "gif", mime: "image/gif", encode: func(buffer *bytes.Buffer) error { return gif.Encode(buffer, imageData, nil) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			buffer := new(bytes.Buffer)
			if err := test.encode(buffer); err != nil {
				t.Fatalf("encode image: %v", err)
			}
			validated, err := ValidatePrivateMessageImage(buffer.Bytes())
			if err != nil || validated.MIMEType != test.mime || !bytes.Equal(validated.Data, buffer.Bytes()) {
				t.Fatalf("validated image = %s/%d bytes, error %v", validated.MIMEType, len(validated.Data), err)
			}
		})
	}
	if _, err := ValidatePrivateMessageImage([]byte("not an image")); err != ErrInvalidMessageImage {
		t.Fatalf("invalid image error = %v", err)
	}
	if _, err := ValidatePrivateMessageImage(bytes.Repeat([]byte("x"), MaxPrivateMessageImageSize+1)); err != ErrMessageImageTooLarge {
		t.Fatalf("oversized image error = %v", err)
	}
}
