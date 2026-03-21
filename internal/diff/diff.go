package diff

import "strings"

type Result struct {
	Changes []Change `json:"changes"`
	Stats   Stats    `json:"stats"`
}

type Change struct {
	Type     string `json:"type"`
	LineNum1 int    `json:"line_num_1"`
	LineNum2 int    `json:"line_num_2"`
	Text1    string `json:"text_1,omitempty"`
	Text2    string `json:"text_2,omitempty"`
}

type Stats struct {
	LinesAdded   int  `json:"lines_added"`
	LinesRemoved int  `json:"lines_removed"`
	FilesEqual   bool `json:"files_equal"`
}

type Options struct {
	IgnoreCase  bool
	IgnoreSpace bool
}

// compares two text strings using the LCS algorithm & returns a structured diff result.
func Compare(text1, text2 string, opts Options) (*Result, error) {
	lines1 := strings.Split(text1, "\n")
	lines2 := strings.Split(text2, "\n")

	changes, stats := computeLCS(lines1, lines2, opts)
	return &Result{Changes: changes, Stats: stats}, nil
}

// implements the Longest Common Subsequence algorithm to generate line-by-line diffs
func computeLCS(lines1, lines2 []string, opts Options) ([]Change, Stats) {
	n := len(lines1)
	m := len(lines2)

	cmp1 := make([]string, n)
	cmp2 := make([]string, m)
	for i, l := range lines1 {
		cmp1[i] = normalize(l, opts)
	}
	for i, l := range lines2 {
		cmp2[i] = normalize(l, opts)
	}

	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			if cmp1[i-1] == cmp2[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] > dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	var rev []Change
	i, j := n, m
	for i > 0 || j > 0 {
		if i > 0 && j > 0 && cmp1[i-1] == cmp2[j-1] {
			rev = append(rev, Change{Type: "equal", LineNum1: i, LineNum2: j, Text1: lines1[i-1], Text2: lines2[j-1]})
			i--
			j--
		} else if j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]) {
			rev = append(rev, Change{Type: "inserted", LineNum2: j, Text2: lines2[j-1]})
			j--
		} else if i > 0 {
			rev = append(rev, Change{Type: "deleted", LineNum1: i, Text1: lines1[i-1]})
			i--
		}
	}

	changes := make([]Change, len(rev))
	for k := range rev {
		changes[k] = rev[len(rev)-1-k]
	}

	var stats Stats
	for _, c := range changes {
		switch c.Type {
		case "deleted":
			stats.LinesRemoved++
		case "inserted":
			stats.LinesAdded++
		}
	}
	stats.FilesEqual = stats.LinesAdded == 0 && stats.LinesRemoved == 0
	return changes, stats
}

func normalize(line string, opts Options) string {
	if opts.IgnoreCase {
		line = strings.ToLower(line)
	}
	if opts.IgnoreSpace {
		line = strings.TrimSpace(line)
	}
	return line
}
