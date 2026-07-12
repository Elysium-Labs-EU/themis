// Package lynis wraps the Lynis security audit tool: running it and
// parsing its report file into structured findings.
package lynis

import (
	"bufio"
	"io"
	"strings"
)

// Finding is one suggestion or warning line from a Lynis report.
type Finding struct {
	TestID      string
	Description string
	Severity    string
	Kind        string // "suggestion" or "warning"
}

const (
	suggestionPrefix = "suggestion[]="
	warningPrefix    = "warning[]="
)

// ParseReport reads a Lynis report.dat stream and extracts suggestion[]
// and warning[] entries. Each entry is pipe-delimited:
// test_id|description|severity|f4.
func ParseReport(r io.Reader) ([]Finding, error) {
	var findings []Finding

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()

		switch {
		case strings.HasPrefix(line, suggestionPrefix):
			findings = append(findings, parseFindingLine(line[len(suggestionPrefix):], "suggestion"))
		case strings.HasPrefix(line, warningPrefix):
			findings = append(findings, parseFindingLine(line[len(warningPrefix):], "warning"))
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return findings, nil
}

func parseFindingLine(body, kind string) Finding {
	fields := strings.Split(body, "|")
	f := Finding{Kind: kind}
	if len(fields) > 0 {
		f.TestID = fields[0]
	}
	if len(fields) > 1 {
		f.Description = fields[1]
	}
	if len(fields) > 2 {
		f.Severity = fields[2]
	}
	return f
}
