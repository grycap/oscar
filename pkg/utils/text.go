package utils

import "strings"

// NormalizeLineEndings converts CRLF and CR sequences to LF to avoid issues with mixed OS line endings.
func NormalizeLineEndings(input string) string {
	// First handle Windows CRLF pairs, then any remaining standalone CR characters.
	input = strings.ReplaceAll(input, "\r\n", "\n")
	return strings.ReplaceAll(input, "\r", "\n")
}
