package imgconv

import (
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"

	"sudosre/internal/util"

	"golang.org/x/image/bmp"
	"golang.org/x/image/draw"
	"golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

type Info struct {
	Path    string `json:"path"`
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	SizeStr string `json:"size_str"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	Format  string `json:"format"`
}

type ConvertResult struct {
	Success       bool   `json:"success"`
	Error         string `json:"error,omitempty"`
	InputPath     string `json:"input_path"`
	OutputPath    string `json:"output_path"`
	InputSize     int64  `json:"input_size"`
	OutputSize    int64  `json:"output_size"`
	InputSizeStr  string `json:"input_size_str"`
	OutputSizeStr string `json:"output_size_str"`
	InputFormat   string `json:"input_format"`
	OutputFormat  string `json:"output_format"`
}

// SupportedFormats lists the output formats the converter can produce.
var SupportedFormats = []string{"png", "jpeg", "gif", "bmp", "tiff"}

// GetInfo returns metadata about an image file.
func GetInfo(path string) (*Info, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	cfg, format, err := image.DecodeConfig(f)
	if err != nil {
		return nil, fmt.Errorf("cannot decode image: %v", err)
	}

	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	return &Info{
		Path: path, Name: filepath.Base(path),
		Size: fi.Size(), SizeStr: util.FormatFileSize(fi.Size()),
		Width: cfg.Width, Height: cfg.Height, Format: format,
	}, nil
}

// Convert reads an image from inputPath, optionally resizes it, encodes in outputFormat,
// and writes it to outputPath.
// quality is used for JPEG (1–100). maxDimension is the maximum longest edge in pixels; 0 means no resize.
// pngCompression is one of: default, fast, best, none (PNG output only).
func Convert(inputPath, outputPath, outputFormat string, quality int, maxDimension int, pngCompression string) (*ConvertResult, error) {
	inputInfo, err := os.Stat(inputPath)
	if err != nil {
		return fail(fmt.Sprintf("Cannot read input file: %v", err)), nil
	}

	inFile, err := os.Open(inputPath)
	if err != nil {
		return fail(fmt.Sprintf("Cannot open input file: %v", err)), nil
	}
	defer inFile.Close()

	img, inputFormat, err := image.Decode(inFile)
	if err != nil {
		return fail(fmt.Sprintf("Cannot decode image: %v", err)), nil
	}

	img = maybeResize(img, maxDimension)

	if quality < 1 {
		quality = 1
	}
	if quality > 100 {
		quality = 100
	}

	outFile, err := os.Create(outputPath)
	if err != nil {
		return fail(fmt.Sprintf("Cannot create output file: %v", err)), nil
	}
	defer outFile.Close()

	switch strings.ToLower(outputFormat) {
	case "png":
		enc := png.Encoder{CompressionLevel: pngCompressionLevel(pngCompression)}
		err = enc.Encode(outFile, img)
	case "jpeg", "jpg":
		err = jpeg.Encode(outFile, img, &jpeg.Options{Quality: quality})
	case "gif":
		err = gif.Encode(outFile, img, nil)
	case "bmp":
		err = bmp.Encode(outFile, img)
	case "tiff", "tif":
		err = tiff.Encode(outFile, img, nil)
	default:
		return fail(fmt.Sprintf("Unsupported output format: %s", outputFormat)), nil
	}

	if err != nil {
		return fail(fmt.Sprintf("Failed to encode image: %v", err)), nil
	}

	outputInfo, err := os.Stat(outputPath)
	if err != nil {
		return fail(fmt.Sprintf("Cannot read output file: %v", err)), nil
	}

	return &ConvertResult{
		Success: true, InputPath: inputPath, OutputPath: outputPath,
		InputSize: inputInfo.Size(), OutputSize: outputInfo.Size(),
		InputSizeStr:  util.FormatFileSize(inputInfo.Size()),
		OutputSizeStr: util.FormatFileSize(outputInfo.Size()),
		InputFormat:   inputFormat, OutputFormat: outputFormat,
	}, nil
}

func pngCompressionLevel(s string) png.CompressionLevel {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "fast", "best-speed", "speed":
		return png.BestSpeed
	case "best", "smallest", "max", "best-compression":
		return png.BestCompression
	case "none", "no", "off":
		return png.NoCompression
	default:
		return png.DefaultCompression
	}
}

func maybeResize(src image.Image, maxDim int) image.Image {
	if maxDim <= 0 || src == nil {
		return src
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return src
	}
	if w <= maxDim && h <= maxDim {
		return src
	}
	nw, nh := scaledSize(w, h, maxDim)
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
	return dst
}

func scaledSize(w, h, maxDim int) (int, int) {
	if w >= h {
		nw := maxDim
		nh := int(math.Round(float64(h) * float64(maxDim) / float64(w)))
		return nw, nh
	}
	nh := maxDim
	nw := int(math.Round(float64(w) * float64(maxDim) / float64(h)))
	return nw, nh
}

func fail(msg string) *ConvertResult {
	return &ConvertResult{Success: false, Error: msg}
}
