package lang

import (
	"regexp"
	"strconv"
	"strings"
)

// SourceCommentPrefix opens a source cite comment.
const SourceCommentPrefix = "//@"

// Cite is one cite of a `//@` line. At and Len are its byte span (repo
// through ranges) within the comment text.
type Cite struct {
	Repo       string
	Path       string
	Start, End int
	At, Len    int
}

var repoRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

// ParseSourceComment parses a `//@ [repo] path[:lines][, …] [-- text]`
// comment (spec §14.1). ok is false for anything else: the line then stays
// an ordinary comment.
func ParseSourceComment(comment string) (cites []Cite, text string, ok bool) {
	body, found := strings.CutPrefix(comment, SourceCommentPrefix)
	if !found {
		return nil, "", false
	}
	head, note, _ := strings.Cut(body, " -- ")
	text = strings.TrimSpace(note)
	offset := len(SourceCommentPrefix)
	var prevRepo, prevPath string
	for _, part := range strings.Split(head, ", ") {
		at := offset + len(part) - len(strings.TrimLeft(part, " "))
		offset += len(part) + len(", ")
		fields := strings.Fields(part)
		var repo, loc string
		switch len(fields) {
		case 1:
			loc = fields[0]
		case 2:
			repo, loc = fields[0], fields[1]
			if !repoRe.MatchString(repo) {
				return nil, "", false
			}
		default:
			return nil, "", false
		}
		span := len(strings.TrimSpace(part))
		path, ranges, hasRanges := strings.Cut(loc, ":")
		switch {
		case path == "" && (prevPath == "" || repo != ""):
			return nil, "", false
		case path == "":
			path, repo = prevPath, prevRepo
		case !strings.ContainsAny(path, "/."):
			return nil, "", false
		}
		prevRepo, prevPath = repo, path
		if !hasRanges {
			cites = append(cites, Cite{Repo: repo, Path: path, At: at, Len: span})
			continue
		}
		for _, r := range strings.Split(ranges, ",") {
			a, b, isRange := strings.Cut(r, "-")
			start, err := strconv.Atoi(a)
			if err != nil || start < 1 {
				return nil, "", false
			}
			end := start
			if isRange {
				if end, err = strconv.Atoi(b); err != nil || end < start {
					return nil, "", false
				}
			}
			cites = append(cites, Cite{Repo: repo, Path: path, Start: start, End: end, At: at, Len: span})
		}
	}
	if len(cites) == 0 {
		return nil, "", false
	}
	return cites, text, true
}
