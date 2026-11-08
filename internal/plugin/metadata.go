package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// MaxCandidates is how many candidates identify may return; the rest are
// dropped.
const MaxCandidates = 20

// Book is what a metadata source is asked to look up: what the user has
// for the book, or what they typed instead. Only these fields leave the
// library, and only for the book the user asked about.
type Book struct {
	Title   string   `json:"title"`
	Authors []string `json:"authors"`
	// Identifiers are as calibre keeps them: "isbn", "goodreads", ….
	Identifiers map[string]string `json:"identifiers"`
	// Language is the book's first language as an ISO 639 code, if known.
	Language string `json:"language,omitempty"`
}

// Identify asks a metadata source for candidates for book, best first.
// A candidate is a Result whose formats may be empty (they are dropped).
// A malformed candidate is dropped with a warning, logged.
func (p *Plugin) Identify(ctx context.Context, book Book) ([]Result, error) {
	if !p.Manifest.IsMetadataSource() {
		return nil, &Error{Plugin: p.Manifest.ID, Call: "identify", Kind: KindInvalid, Msg: "isn’t a metadata source"}
	}
	book.Title = strings.TrimSpace(book.Title)
	book.Authors = slices.DeleteFunc(slices.Clone(book.Authors), func(a string) bool { return strings.TrimSpace(a) == "" })
	if book.Authors == nil {
		book.Authors = []string{}
	}
	ids := map[string]string{}
	for k, v := range book.Identifiers {
		if k, v = strings.ToLower(strings.TrimSpace(k)), strings.TrimSpace(v); k != "" && v != "" {
			ids[k] = v
		}
	}
	book.Identifiers = ids
	if book.Title == "" && len(ids) == 0 {
		return nil, &Error{Plugin: p.Manifest.ID, Call: "identify", Kind: KindInvalid, Msg: "nothing to look up: no title and no identifiers"}
	}
	raw, err := p.call(ctx, "identify", book)
	if err != nil {
		return nil, err
	}
	out, warnings, err := parseCandidates(raw)
	for _, w := range warnings {
		p.logf("[%s] warning: identify returned %s", p.Manifest.ID, w)
	}
	if err != nil {
		return nil, &Error{Plugin: p.Manifest.ID, Call: "identify", Kind: KindInvalid, Msg: err.Error()}
	}
	return out, nil
}

// parseCandidates reads what identify returned: an array of results.
func parseCandidates(raw json.RawMessage) ([]Result, []string, error) {
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil || (items == nil && strings.TrimSpace(string(raw)) != "[]") {
		problem := "null"
		if err != nil {
			problem = jsonProblem(err)
		}
		return nil, nil, fmt.Errorf("returned %s; want an array of candidates, [] when nothing is found", problem)
	}
	var warnings []string
	if len(items) > MaxCandidates {
		warnings = append(warnings, fmt.Sprintf("%d candidates; only the first %d are kept", len(items), MaxCandidates))
		items = items[:MaxCandidates]
	}
	out := []Result{}
	seen := map[string]bool{}
	for i, item := range items {
		at := fmt.Sprintf("[%d]", i)
		var r Result
		if err := strictJSON(item, &r); err != nil {
			var unknown *unknownFieldError
			if !errors.As(err, &unknown) {
				warnings = append(warnings, fmt.Sprintf("%s: %s; dropped", at, jsonProblem(err)))
				continue
			}
			warnings = append(warnings, fmt.Sprintf("%s: %s; ignored", at, jsonProblem(err)))
		}
		r.Formats, r.Extra = nil, nil // a candidate is details only
		warn, err := r.checkDetails()
		for _, w := range warn {
			warnings = append(warnings, at+"."+w)
		}
		if err == nil && seen[r.ID] {
			err = fmt.Errorf("id: %q is used by an earlier candidate", r.ID)
		}
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s.%v; dropped", at, err))
			continue
		}
		seen[r.ID] = true
		r.Identifiers = normalIdentifiers(r.Identifiers)
		out = append(out, r)
	}
	return out, warnings, nil
}

func normalIdentifiers(ids map[string]string) map[string]string {
	if len(ids) == 0 {
		return nil
	}
	out := make(map[string]string, len(ids))
	for _, k := range slices.Sorted(maps.Keys(ids)) {
		if nk, v := strings.ToLower(strings.TrimSpace(k)), strings.TrimSpace(ids[k]); nk != "" && v != "" && !strings.ContainsAny(nk, ":,") {
			out[nk] = v
		}
	}
	return out
}
