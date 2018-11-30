package rty

import (
	"io"
	"unicode"
)

// A tokenizer that breaks a string up by spaces.
//
// Ideally, we'd use the table-based algorithm defined in:
// http://www.unicode.org/reports/tr14/
// like this package does:
// https://godoc.org/github.com/gorilla/i18n/linebreak
// but I didn't find a good implementation of that algorithm in Go
// (the one above is half-implemented and doesn't work for
// the most basic things).
//
// This is a half-assed implementation that should have a similar interface
// to a "real" implementation.
type Tokenizer struct {
	runes []rune
	pos   int
}

func NewTokenizer(s string) *Tokenizer {
	return &Tokenizer{
		runes: []rune(s),
		pos:   0,
	}
}

func (t *Tokenizer) Next() ([]rune, error) {
	if t.pos >= len(t.runes) {
		return nil, io.EOF
	}

	firstRune := t.runes[t.pos]
	isSpace := unicode.IsSpace(firstRune)
	result := []rune{t.runes[t.pos]}
	t.pos++

	for t.pos < len(t.runes) {
		nextRune := t.runes[t.pos]
		isNextSpace := unicode.IsSpace(nextRune)
		if isNextSpace || isSpace {
			return result, nil
		}

		result = append(result, nextRune)
		t.pos++
	}

	return result, nil
}
