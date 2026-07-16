package homepage

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"testing"
)

func TestValidateImageDataRejectsFormatMismatchAndUnsafeDimensionsExactly(t *testing.T) {
	valid := pngConfigBytes(1, 1)
	if err := validateImageData(valid, ".png"); err != nil {
		t.Fatalf("valid PNG config error=%v, want nil", err)
	}
	if err := validateImageData(valid, ".webp"); !errors.Is(err, errInvalidImageFormat) {
		t.Fatalf("format mismatch error=%v, want errInvalidImageFormat", err)
	}
	if err := validateImageData(pngConfigBytes(MaxImageDimension+1, 1), ".png"); !errors.Is(err, errImageDimensions) {
		t.Fatalf("oversized width error=%v, want errImageDimensions", err)
	}
	if err := validateImageData(pngConfigBytes(MaxImageDimension, MaxImagePixels/MaxImageDimension+1), ".png"); !errors.Is(err, errImageDimensions) {
		t.Fatalf("oversized pixel count error=%v, want errImageDimensions", err)
	}
}

func TestExpectedImageFormatMapsEveryAcceptedExtensionExactly(t *testing.T) {
	for extension, want := range map[string]string{
		".png": "png", ".jpg": "jpeg", ".jpeg": "jpeg", ".webp": "webp", ".gif": "",
	} {
		if got := expectedImageFormat(extension); got != want {
			t.Fatalf("expectedImageFormat(%q)=%q, want %q", extension, got, want)
		}
	}
}

func pngConfigBytes(width, height int) []byte {
	var out bytes.Buffer
	out.Write([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})
	data := make([]byte, 13)
	binary.BigEndian.PutUint32(data[0:4], uint32(width))
	binary.BigEndian.PutUint32(data[4:8], uint32(height))
	data[8] = 8
	data[9] = 2
	writePNGChunk(&out, "IHDR", data)
	return out.Bytes()
}

func writePNGChunk(out *bytes.Buffer, kind string, data []byte) {
	_ = binary.Write(out, binary.BigEndian, uint32(len(data)))
	out.WriteString(kind)
	out.Write(data)
	checksum := crc32.NewIEEE()
	_, _ = checksum.Write([]byte(kind))
	_, _ = checksum.Write(data)
	_ = binary.Write(out, binary.BigEndian, checksum.Sum32())
}
