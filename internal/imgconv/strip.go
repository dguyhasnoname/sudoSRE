package imgconv

import (
	"fmt"
	"strings"
)

// re-encodes the image so EXIF, ICC profiles, and most auxiliary PNG/WebP chunks
// are not carried over (decode → RGBA → encode). JPEG uses high quality to limit recompression.
// WebP input is written as PNG (no WebP encoder in this app). Animated GIF becomes a single frame.
func StripMetadata(inputPath, outputPath string) (*ConvertResult, error) {
	info, err := GetInfo(inputPath)
	if err != nil {
		return fail(fmt.Sprintf("Cannot read image: %v", err)), nil
	}

	outFmt := formatForStrip(info.Format)
	// JPEG: 95 minimizes visible loss on strip; other formats ignore quality.
	const jpegQuality = 95
	return Convert(inputPath, outputPath, outFmt, jpegQuality, 0, "default")
}

// map detected decode format to a supported output format
func formatForStrip(detected string) string {
	f := strings.ToLower(strings.TrimSpace(detected))
	switch f {
	case "jpeg", "jpg":
		return "jpeg"
	case "png":
		return "png"
	case "gif":
		return "gif"
	case "bmp":
		return "bmp"
	case "tiff", "tif":
		return "tiff"
	case "webp":
		// no WebP encoder; PNG preserves transparency from decoded bitmap.
		return "png"
	default:
		// safe fallback if decoder recognized something odd but image.Decode works in Convert
		return "png"
	}
}
