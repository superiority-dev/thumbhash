package testimage

import (
	_ "embed"
	"encoding/hex"
	"fmt"
	"math"
	"strconv"
	"strings"
)

//go:embed testdata/reference.txt
var reference string

// Result is what the reference encoder produced for one case.
type Result struct {
	ID   string
	Hash []byte

	// Unrounded is each value the reference encoder rounded to build the hash,
	// taken just before rounding, in the reference's order.
	Unrounded []float64
}

// Expected is a parsed reference file.
type Expected struct {
	header  []string
	Results []Result
}

// Reference parses testdata/reference.txt: header lines start with "# ",
// every other line is
//
//	<case id> <hash in hex> <unrounded> ...
func Reference() (Expected, error) {
	header, results, err := parseFile(reference, parseResult)
	if err != nil {
		return Expected{}, err
	}

	return Expected{header: header, Results: results}, nil
}

// Value returns the rest of the header line starting with key, or "" if
// there is none.
func (e Expected) Value(key string) string {
	for _, h := range e.header {
		if v, ok := strings.CutPrefix(h, key+" "); ok {
			return v
		}
	}

	return ""
}

// parseFile splits a fixture file into its header lines, which start with
// "# ", and its results, one a line, each read by parse.
func parseFile[R any](text string, parse func(string) (R, error)) ([]string, []R, error) {
	var (
		header  []string
		results []R
	)

	for n, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
		if h, ok := strings.CutPrefix(line, "# "); ok {
			header = append(header, h)

			continue
		}

		r, err := parse(line)
		if err != nil {
			return nil, nil, fmt.Errorf("line %d: %w", n+1, err)
		}

		results = append(results, r)
	}

	return header, results, nil
}

func parseResult(line string) (Result, error) {
	f := strings.Fields(line)
	if len(f) < 3 {
		return Result{}, fmt.Errorf("want an ID, a hash and unrounded values, got %q", line)
	}

	hash, err := hex.DecodeString(f[1])
	if err != nil {
		return Result{}, err
	}

	r := Result{ID: f[0], Hash: hash, Unrounded: make([]float64, len(f)-2)}

	for i, s := range f[2:] {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return Result{}, err
		}

		if math.IsNaN(v) || math.IsInf(v, 0) {
			return Result{}, fmt.Errorf("unrounded value %d is %v", i, v)
		}

		r.Unrounded[i] = v
	}

	return r, nil
}
