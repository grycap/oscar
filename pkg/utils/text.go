package utils

import (
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// NormalizeLineEndings converts CRLF and CR sequences to LF to avoid issues with mixed OS line endings.
func NormalizeLineEndings(input string) string {
	// First handle Windows CRLF pairs, then any remaining standalone CR characters.
	input = strings.ReplaceAll(input, "\r\n", "\n")
	return strings.ReplaceAll(input, "\r", "\n")
}

func RemoveAccents(input string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	output, _, err := transform.String(t, input)
	if err != nil {
		fmt.Printf("Error during transformation: %v\n", err)
		return input
	}
	return output
}
