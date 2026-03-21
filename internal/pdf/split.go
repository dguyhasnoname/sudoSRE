package pdf

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

const (
	SplitModeSegment = "segment" // one output PDF per comma-separated segment
	SplitModeCombine = "combine" // single PDF with pages in segment order
)

var (
	reSplitSingle = regexp.MustCompile(`^\s*(\d+)\s*$`)
	reSplitRange  = regexp.MustCompile(`^\s*(\d+)\s*-\s*(\d+)\s*$`)
)

func defaultSplitConf() *model.Configuration {
	c := model.NewDefaultConfiguration()
	c.ValidationMode = model.ValidationRelaxed
	return c
}

// returns the number of pages in a PDF (1-based indexing for user-facing APIs)
func PageCount(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	ctx, err := api.ReadAndValidate(f, defaultSplitConf())
	if err != nil {
		return 0, err
	}
	return ctx.PageCount, nil
}

// splits a user page specification into comma-separated segments.
// each segment is either "N" or "N-M" (inclusive, 1-based). whitespace is ignored.
func ParsePageSpec(spec string) ([]string, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, fmt.Errorf("page specification is empty")
	}
	raw := strings.Split(spec, ",")
	out := make([]string, 0, len(raw))
	for _, part := range raw {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("empty segment (check commas)")
		}
		norm, err := normalizeSegment(part)
		if err != nil {
			return nil, err
		}
		out = append(out, norm)
	}
	return out, nil
}

func normalizeSegment(s string) (string, error) {
	if m := reSplitRange.FindStringSubmatch(s); m != nil {
		return fmt.Sprintf("%s-%s", m[1], m[2]), nil
	}
	if m := reSplitSingle.FindStringSubmatch(s); m != nil {
		return m[1], nil
	}
	return "", fmt.Errorf("invalid segment %q: use N or N-M (e.g. 3 or 12-15)", s)
}

// returns inclusive page numbers for one segment in ascending order
func expandSegment(seg string, pageCount int) ([]int, error) {
	if m := reSplitSingle.FindStringSubmatch(seg); m != nil {
		p, err := strconv.Atoi(m[1])
		if err != nil {
			return nil, err
		}
		if p < 1 || p > pageCount {
			return nil, fmt.Errorf("page %d out of range (1-%d)", p, pageCount)
		}
		return []int{p}, nil
	}
	if m := reSplitRange.FindStringSubmatch(seg); m != nil {
		a, err1 := strconv.Atoi(m[1])
		b, err2 := strconv.Atoi(m[2])
		if err1 != nil || err2 != nil {
			return nil, fmt.Errorf("invalid range %q", seg)
		}
		if a > b {
			return nil, fmt.Errorf("invalid range %q: start is greater than end", seg)
		}
		if a < 1 || b > pageCount {
			return nil, fmt.Errorf("range %q out of bounds (document has %d pages)", seg, pageCount)
		}
		n := b - a + 1
		pages := make([]int, n)
		for i := 0; i < n; i++ {
			pages[i] = a + i
		}
		return pages, nil
	}
	return nil, fmt.Errorf("invalid segment %q", seg)
}

// expands all segments in order for combine mode (Mode B)
func expandSpecCombine(segments []string, pageCount int) ([]int, error) {
	var out []int
	for _, seg := range segments {
		pages, err := expandSegment(seg, pageCount)
		if err != nil {
			return nil, err
		}
		out = append(out, pages...)
	}
	return out, nil
}

// returns human-readable warnings when segments share pages (Mode A)
func segmentOverlapWarnings(segments []string, pageCount int) ([]string, error) {
	sets := make([]map[int]struct{}, 0, len(segments))
	for _, seg := range segments {
		pages, err := expandSegment(seg, pageCount)
		if err != nil {
			return nil, err
		}
		m := make(map[int]struct{}, len(pages))
		for _, p := range pages {
			m[p] = struct{}{}
		}
		sets = append(sets, m)
	}
	var warns []string
	for i := 0; i < len(sets); i++ {
		for j := i + 1; j < len(sets); j++ {
			for p := range sets[i] {
				if _, ok := sets[j][p]; ok {
					warns = append(warns, fmt.Sprintf("page %d appears in segment %d and segment %d", p, i+1, j+1))
					break
				}
			}
		}
	}
	return warns, nil
}

// returned by SplitPDF
type SplitResult struct {
	Success     bool     `json:"success"`
	Error       string   `json:"error,omitempty"`
	InputPath   string   `json:"input_path"`
	PageCount   int      `json:"page_count"`
	Mode        string   `json:"mode"`
	OutputPaths []string `json:"output_paths,omitempty"`
	Warnings    []string `json:"warnings,omitempty"`
}

// splits a PDF using pdfcpu (embedded)
// mode is SplitModeSegment (one file per comma-separated segment) or SplitModeCombine (single merged PDF).
// for segment mode, outputPath is a directory; for combine mode, outputPath is a file path
func SplitPDF(inputPath, pageSpec, mode, outputPath string) (*SplitResult, error) {
	inputPath = strings.TrimSpace(inputPath)
	pageSpec = strings.TrimSpace(pageSpec)
	mode = strings.TrimSpace(strings.ToLower(mode))
	outputPath = strings.TrimSpace(outputPath)

	if inputPath == "" || pageSpec == "" || outputPath == "" {
		return failSplit("input path, page specification, and output path are required"), nil
	}
	if mode != SplitModeSegment && mode != SplitModeCombine {
		return failSplit("mode must be \"segment\" or \"combine\""), nil
	}

	segments, err := ParsePageSpec(pageSpec)
	if err != nil {
		return failSplit(err.Error()), nil
	}

	n, err := PageCount(inputPath)
	if err != nil {
		return failSplit(fmt.Sprintf("could not read PDF: %v", err)), nil
	}

	switch mode {
	case SplitModeSegment:
		return splitSegment(inputPath, segments, n, outputPath)
	case SplitModeCombine:
		return splitCombine(inputPath, segments, n, outputPath)
	default:
		return failSplit("unknown mode"), nil
	}
}

func failSplit(msg string) *SplitResult {
	return &SplitResult{Success: false, Error: msg}
}

func splitSegment(inputPath string, segments []string, pageCount int, outDir string) (*SplitResult, error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return failSplit(fmt.Sprintf("cannot create output folder: %v", err)), nil
	}

	warns, err := segmentOverlapWarnings(segments, pageCount)
	if err != nil {
		return failSplit(err.Error()), nil
	}

	conf := defaultSplitConf()
	base := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	if base == "" {
		base = "document"
	}

	var outs []string
	for i, seg := range segments {
		if _, err := expandSegment(seg, pageCount); err != nil {
			return failSplit(err.Error()), nil
		}
		outPath := filepath.Join(outDir, fmt.Sprintf("%s_part%02d.pdf", base, i+1))
		sel := []string{seg}
		if err := api.TrimFile(inputPath, outPath, sel, conf); err != nil {
			return failSplit(fmt.Sprintf("split failed for segment %q: %v", seg, err)), nil
		}
		outs = append(outs, outPath)
	}

	return &SplitResult{
		Success:     true,
		InputPath:   inputPath,
		PageCount:   pageCount,
		Mode:        SplitModeSegment,
		OutputPaths: outs,
		Warnings:    warns,
	}, nil
}

func splitCombine(inputPath string, segments []string, pageCount int, outFile string) (*SplitResult, error) {
	if filepath.Ext(strings.ToLower(outFile)) == "" {
		outFile += ".pdf"
	}

	pageNrs, err := expandSpecCombine(segments, pageCount)
	if err != nil {
		return failSplit(err.Error()), nil
	}
	if len(pageNrs) == 0 {
		return failSplit("no pages selected"), nil
	}

	f, err := os.Open(inputPath)
	if err != nil {
		return failSplit(fmt.Sprintf("open input: %v", err)), nil
	}
	defer f.Close()

	conf := defaultSplitConf()
	conf.Cmd = model.TRIM

	ctx, err := api.ReadValidateAndOptimize(f, conf)
	if err != nil {
		return failSplit(fmt.Sprintf("read PDF: %v", err)), nil
	}

	ctxDest, err := pdfcpu.ExtractPages(ctx, pageNrs, false)
	if err != nil {
		return failSplit(fmt.Sprintf("extract pages: %v", err)), nil
	}

	if err := api.WriteContextFile(ctxDest, outFile); err != nil {
		return failSplit(fmt.Sprintf("write output: %v", err)), nil
	}

	return &SplitResult{
		Success:     true,
		InputPath:   inputPath,
		PageCount:   pageCount,
		Mode:        SplitModeCombine,
		OutputPaths: []string{outFile},
	}, nil
}
