package bot

import (
	"regexp"
	"strings"
	"unicode"
)

type entry struct {
	Word        string
	Translation string
	Note        string
	Groups      []string
}

var (
	tagRe   = regexp.MustCompile(`(?:^|\s)#([\p{L}\p{N}_-]+)`)
	sepRe   = regexp.MustCompile(`\s+[-–—]\s+|\s*=\s*|\t`)
	parenRe = regexp.MustCompile(`\([^)]*\)`)
	starRe  = regexp.MustCompile(`\*([^*]+)\*`)
)

// parseEntry parses one line of the form "word - translation | note #group1 #group2".
// The separator may also be "—", "–", "=" or a tab; note and groups are optional.
func parseEntry(line string) (entry, bool) {
	var e entry
	for _, m := range tagRe.FindAllStringSubmatch(line, -1) {
		e.Groups = append(e.Groups, strings.ToLower(m[1]))
	}
	line = tagRe.ReplaceAllString(line, " ")

	if i := strings.Index(line, "|"); i >= 0 {
		e.Note = strings.TrimSpace(line[i+1:])
		line = line[:i]
	}

	loc := sepRe.FindStringIndex(line)
	if loc == nil {
		return e, false
	}
	e.Word = strings.TrimSpace(line[:loc[0]])
	e.Translation = strings.TrimSpace(line[loc[1]:])
	return e, e.Word != "" && e.Translation != ""
}

// checkAnswer reports whether input matches expected or one of its variants
// ("дом, здание; house/home"), ignoring case, ё/е, punctuation and "(…)" hints.
// When expected marks the key part with asterisks ("he has *no beard*"), typing
// just that part is enough; the whole phrase is accepted too.
func checkAnswer(input, expected string) bool {
	got := normalize(input)
	if got == "" {
		return false
	}
	answers := []string{strings.ReplaceAll(expected, "*", "")}
	if marked := starRe.FindAllStringSubmatch(expected, -1); marked != nil {
		var parts []string
		for _, m := range marked {
			parts = append(parts, m[1])
		}
		answers = append(answers, parts...)
		if len(parts) > 1 {
			answers = append(answers, strings.Join(parts, " "))
		}
	}
	for _, a := range answers {
		variants := strings.FieldsFunc(a, func(r rune) bool { return r == ',' || r == ';' || r == '/' })
		for _, v := range append(variants, a) {
			if normalize(v) == got {
				return true
			}
		}
	}
	return false
}

func normalize(s string) string {
	s = parenRe.ReplaceAllString(strings.ToLower(s), " ")
	s = strings.ReplaceAll(s, "ё", "е")
	s = strings.TrimFunc(s, func(r rune) bool { return unicode.IsPunct(r) || unicode.IsSpace(r) })
	return strings.Join(strings.Fields(s), " ")
}
