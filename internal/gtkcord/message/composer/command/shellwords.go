package command

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
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

	if len(firstWord.Parts) == 0 {
		return "", nil
	}

	var b strings.Builder

	if err := shPartsLit(firstWord.Parts, &b); err != nil {
		return b.String(), err
	}

	return b.String(), nil
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

func shPartsLit(parts []syntax.WordPart, b *strings.Builder) error {
	for _, part := range parts {
		switch part := part.(type) {
		case *syntax.DblQuoted:
			if err := shPartsLit(part.Parts, b); err != nil {
				return err
			}
		case *syntax.SglQuoted:
			b.WriteString(part.Value)
		case *syntax.Lit:
			b.WriteString(part.Value)
		default:
			return fmt.Errorf("word part type %T not allowed", part)
		}
	}
	return nil
}
