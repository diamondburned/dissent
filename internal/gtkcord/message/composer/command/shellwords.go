package command

import (
	"strings"

	"github.com/pkg/errors"
	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/syntax"
)

var parser = syntax.NewParser(
	syntax.Variant(syntax.LangBash),
)

// parseSingleWord parses the given text string for a single shell word string.
// The returned string will have all its quoting already resolved.
func parseSingleWord(text string) (string, error) {
	var firstWord *syntax.Word
	err := parser.Words(strings.NewReader(text), func(word *syntax.Word) bool {
		firstWord = word
		return false
	})
	if err != nil {
		return "", errors.Wrap(err, "cannot parse for first shell word")
	}

	return shLiteral(firstWord)
}

// isInShellWord returns true if the given index is within a shell word.
func isInShellWord(text string) bool {
	err := parser.Words(strings.NewReader(text), func(word *syntax.Word) bool {
		return false
	})
	// ???: we're supposed to be using parser.IsIncomplete to check for this,
	// but this function returns false even when we have an incomplete word.
	// See https://go.dev/play/p/qE3_HR3SjML.
	return err != nil
}

// shLiteral returns the literal string representation of the given shell word.
func shLiteral(word *syntax.Word) (string, error) {
	return expand.Literal(nil, word)
}
