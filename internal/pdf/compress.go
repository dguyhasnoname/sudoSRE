package pdf

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"sudosre/internal/util"
)

type Info struct {
	Path    string `json:"path"`
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	SizeStr string `json:"size_str"`
}

type CompressResult struct {
	Success       bool    `json:"success"`
	Error         string  `json:"error,omitempty"`
	InputPath     string  `json:"input_path"`
	OutputPath    string  `json:"output_path"`
	InputSize     int64   `json:"input_size"`
	OutputSize    int64   `json:"output_size"`
	InputSizeStr  string  `json:"input_size_str"`
	OutputSizeStr string  `json:"output_size_str"`
	Reduction     float64 `json:"reduction"`
}

// returns metadata about a PDF file
func GetInfo(path string) (*Info, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	return &Info{
		Path:    path,
		Name:    filepath.Base(path),
		Size:    fi.Size(),
		SizeStr: util.FormatFileSize(fi.Size()),
	}, nil
}

// quality levels: fast, screen (72 dpi), ebook (150 dpi), printer (300 dpi), prepress (300 dpi)
func Compress(inputPath, outputPath, quality string) (*CompressResult, error) {
	gsPath := findGhostscript()
	if gsPath == "" {
		return fail("Ghostscript not found. Please install it with: brew install ghostscript"), nil
	}

	inputInfo, err := os.Stat(inputPath)
	if err != nil {
		return fail(fmt.Sprintf("Cannot read input file: %v", err)), nil
	}

	args := buildArgs(inputPath, outputPath, quality)
	cmd := exec.Command(gsPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fail(fmt.Sprintf("Compression failed: %v - %s", err, string(output))), nil
	}

	outputInfo, err := os.Stat(outputPath)
	if err != nil {
		return fail(fmt.Sprintf("Cannot read output file: %v", err)), nil
	}

	inSize := inputInfo.Size()
	outSize := outputInfo.Size()
	return &CompressResult{
		Success:       true,
		InputPath:     inputPath,
		OutputPath:    outputPath,
		InputSize:     inSize,
		OutputSize:    outSize,
		InputSizeStr:  util.FormatFileSize(inSize),
		OutputSizeStr: util.FormatFileSize(outSize),
		Reduction:     float64(inSize-outSize) / float64(inSize) * 100,
	}, nil
}

func fail(msg string) *CompressResult {
	return &CompressResult{Success: false, Error: msg}
}

func findGhostscript() string {
	if p, err := exec.LookPath("gs"); err == nil {
		return p
	}
	for _, p := range []string{"/opt/homebrew/bin/gs", "/usr/local/bin/gs"} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func buildArgs(inputPath, outputPath, quality string) []string {
	if quality == "fast" {
		return []string{
			"-sDEVICE=pdfwrite",
			"-dCompatibilityLevel=1.4",
			"-dPDFSETTINGS=/screen",
			"-dNOPAUSE", "-dQUIET", "-dBATCH",
			"-dDetectDuplicateImages=false",
			"-dCompressFonts=true",
			"-dSubsetFonts=true",
			"-dColorImageDownsampleType=/Subsample",
			"-dGrayImageDownsampleType=/Subsample",
			"-dMonoImageDownsampleType=/Subsample",
			"-dOptimize=true",
			"-dFastWebView=false",
			"-r72",
			fmt.Sprintf("-sOutputFile=%s", outputPath),
			inputPath,
		}
	}

	settings := map[string]string{
		"screen": "/screen", "ebook": "/ebook",
		"printer": "/printer", "prepress": "/prepress",
	}
	s, ok := settings[quality]
	if !ok {
		s = "/ebook"
	}
	return []string{
		"-sDEVICE=pdfwrite",
		"-dCompatibilityLevel=1.4",
		fmt.Sprintf("-dPDFSETTINGS=%s", s),
		"-dNOPAUSE", "-dQUIET", "-dBATCH",
		"-dDetectDuplicateImages=true",
		"-dCompressFonts=true",
		"-dSubsetFonts=true",
		fmt.Sprintf("-sOutputFile=%s", outputPath),
		inputPath,
	}
}
