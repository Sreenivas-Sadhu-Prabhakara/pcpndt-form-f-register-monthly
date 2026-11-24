package formf

import (
	"regexp"
	"sort"
	"strings"
)

// What this register will not hold.
//
// The Act this register serves exists for one reason: to stop the selection of
// a child before birth and the killing of unborn girls that follows from it.
// A tool built around its paperwork has to be built so that it cannot become
// part of the thing the Act prohibits. Two rules follow, and both are enforced
// in code rather than promised in a README.
//
// The first rule is structural. There is no field anywhere in this register
// for the sex of a foetus. Not a column, not a coded value, not a dropdown,
// not an optional extra, not a hidden note. The prescribed field catalogue in
// fields.go is the complete and only set of things this tool stores, and
// nothing in it asks, infers or derives it. A test walks that catalogue, every
// label, every printed column heading and every line of generated output and
// fails the build if any of them so much as names the subject.
//
// The second rule covers free text, which is where a prohibition leaks. A
// findings box will take whatever is typed into it. So every free-text field
// is screened before a record is stored, and a record carrying a screened term
// is refused — named term by named term — rather than saved with a warning.
//
// The screening is deliberately narrow. Clinical obstetric language is full of
// words like "fetal heart rate", "fetal biometry" and "beta hCG", and a guard
// that refused those would be a guard nobody could keep switched on. So the
// vocabulary is the words that have no other use in a scan note, plus the
// karyotype shorthand that would carry the same meaning in two characters. It
// is not screening for intent and it is no substitute for the sonologist's own
// signed declaration: a word list cannot stop a person determined to misuse a
// tool. What it does stop is this register quietly becoming the place where
// such a thing gets written down.

// screenedWords is the screening vocabulary. It is matched whole-word and
// case-insensitively, so "sonologist", "sonography", "reason" and "person" are
// untouched, and so are the many legitimate uses of "fetal" and "foetal".
//
// Each entry earns its place by having no ordinary use in a scan note:
//   - the plain English terms for the subject and for a child of either kind;
//   - "xy", half of the karyotype shorthand that says the same thing in two
//     characters and would otherwise slip past an English word list;
//   - the common Hindi and Marathi words for a boy and a girl, because a note
//     typed in a clinic is as likely to be in those as in English.
//
// "beta" is deliberately absent: beta hCG is a routine obstetric result and a
// guard that rejected it would be turned off within a week.
var screenedWords = []string{
	"sex", "sexes", "gender", "genders",
	"male", "males", "female", "females",
	"boy", "boys", "girl", "girls",
	"son", "sons", "daughter", "daughters",
	"xy",
	"ladka", "ladke", "ladki", "ladkiyan",
	"beti", "betiyan", "putra", "putri",
	"mulga", "mulgi",
}

var screenedPattern = regexp.MustCompile(`(?i)\b(` + strings.Join(screenedWords, "|") + `)\b`)

// karyotypePattern is the other half of the karyotype shorthand, and it needs
// a rule of its own.
//
// "xy" has no other use and sits in the word list. Its counterpart does not:
// "(xx)" is how Form F numbers the twentieth of the indications it prints at
// item 10, and a guard that refused that would refuse the form's own numbering.
// So this half is matched only with the chromosome count in front of it, in the
// form a karyotype is actually written — "46,XX" — which is specific enough to
// mean one thing and nothing else.
//
// This is a known and deliberate hole: a bare "XX" typed on its own is not
// caught. The declaration the sonologist signs, not a word list, is what
// carries the obligation here; the word list only stops this register becoming
// the place such a thing gets written down.
var karyotypePattern = regexp.MustCompile(`(?i)\b\d{1,2}\s*,\s*xx\b`)

// ScreenedTerms returns the screening vocabulary, so that the documentation
// can state exactly what is refused instead of describing it vaguely.
func ScreenedTerms() []string {
	out := make([]string, len(screenedWords))
	copy(out, screenedWords)
	return out
}

// ScreenText returns every screened term found in s, lowercased, deduplicated
// and in order of first appearance. An empty result means the text is clear.
//
// Screening applies to the free-text clinical fields — the ages of the living
// children, the indication in words, the other procedure, the result, the
// termination detail and the remarks. It is not applied to names, addresses or
// dates, where a surname or a place name could collide with the vocabulary and
// where there is nothing to encode anyway.
func ScreenText(s string) []string {
	type hit struct {
		at   int
		term string
	}
	var hits []hit
	for _, loc := range screenedPattern.FindAllStringIndex(s, -1) {
		hits = append(hits, hit{loc[0], strings.ToLower(s[loc[0]:loc[1]])})
	}
	for _, loc := range karyotypePattern.FindAllStringIndex(s, -1) {
		// Canonicalise "46, XX" and "46,xx" to one term, so that a
		// refusal names the same thing however it was typed.
		term := strings.ToLower(s[loc[0]:loc[1]])
		term = strings.Join(strings.Fields(term), "")
		hits = append(hits, hit{loc[0], term})
	}
	if len(hits) == 0 {
		return nil
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].at < hits[j].at })

	seen := make(map[string]bool, len(hits))
	out := make([]string, 0, len(hits))
	for _, h := range hits {
		if seen[h.term] {
			continue
		}
		seen[h.term] = true
		out = append(out, h.term)
	}
	return out
}

// TextIsClear reports whether s carries no screened term.
func TextIsClear(s string) bool { return len(ScreenText(s)) == 0 }
