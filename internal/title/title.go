// Package title turns an agent's first real request into a short workspace
// title. The rules come from the workspace-title feature (odd/tasks/
// workspace-title.md): one title per workspace, the user's own name always
// wins, 2 to 5 words capped at 34 characters, in the language of the request.
//
// The heuristic here is the last-resort source (S4): a caller that already
// redacted a title with the session's model passes that text as the request,
// and From passes it through unchanged when it fits. The binary itself never
// calls a model.
package title

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// MaxLen is the title cap in runes: "máx. 34 caracteres".
const MaxLen = 34

// MinRequestLen is the eligibility floor in runes: "un pedido de al menos 12
// caracteres".
const MinRequestLen = 12

// MaxWords is the title's word ceiling: "2 a 5 palabras".
const MaxWords = 5

// SourceID is the display-only metadata source the title is reported under:
// "herdr workspace report-metadata <id> --source herdr-utils --token
// title=<text>". The token shows up as tokens.title without touching the
// workspace label.
const SourceID = "herdr-utils"

// Eligible reports whether a request is long enough to title a workspace.
func Eligible(request string) bool {
	return utf8.RuneCountInString(strings.TrimSpace(request)) >= MinRequestLen
}

// From builds the heuristic title: collapse whitespace, keep the first words
// within the 34-rune cap, and stop at MaxWords. A request that already fits —
// for example a model-redacted title passed in as the request — comes back
// unchanged.
func From(request string) string {
	words := strings.Fields(request)
	if len(words) == 0 {
		return ""
	}
	if utf8.RuneCountInString(strings.Join(words, " ")) <= MaxLen && len(words) <= MaxWords {
		return strings.Join(words, " ")
	}

	title := make([]string, 0, MaxWords)
	runes := 0
	for _, word := range words {
		if len(title) >= MaxWords {
			break
		}
		// A first word alone longer than the cap truncates to the cap.
		if len(title) == 0 && utf8.RuneCountInString(word) > MaxLen {
			return truncate(word)
		}
		if len(title) > 0 && runes+1+utf8.RuneCountInString(word) > MaxLen {
			break
		}
		title = append(title, word)
		runes += utf8.RuneCountInString(word)
		if len(title) > 1 {
			runes++
		}
	}
	built := truncate(strings.Join(title, " "))
	// The example title is "Fix login redirect loop": a heuristic title opens
	// capitalized without rewriting the rest of the words.
	first, size := utf8.DecodeRuneInString(built)
	return string(unicode.ToUpper(first)) + built[size:]
}

// truncate cuts text at MaxLen runes without splitting one.
func truncate(text string) string {
	runes := []rune(text)
	if len(runes) <= MaxLen {
		return text
	}
	return string(runes[:MaxLen])
}

// Skip reports whether the workspace must keep its current name: it already
// carries a title (once per workspace, S1) or its label stopped matching the
// default herdr gave it (a manual rename wins, S3). An empty defaultLabel —
// no agent cwd to derive the default from — cannot prove a manual rename, so
// the once-only check alone decides.
func Skip(existingTitle, label, defaultLabel string) bool {
	if existingTitle != "" {
		return true
	}
	return defaultLabel != "" && label != defaultLabel
}
