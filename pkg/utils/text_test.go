package utils

import "testing"

func TestRemoveAccents(t *testing.T) {
	tests := map[string]struct {
		input    string
		expected string
	}{
		"empty string": {
			input:    "",
			expected: "",
		},
		"string without accents": {
			input:    "hello",
			expected: "hello",
		},
		"string with accents": {
			input:    "café",
			expected: "cafe",
		},
		"string with multiple accents": {
			input:    "àéìóú",
			expected: "aeiou",
		},
		"string with ñ": {
			input:    "ñ",
			expected: "n",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := RemoveAccents(tc.input)
			if got != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

func TestNormalizeLineEndings(t *testing.T) {
	tests := map[string]struct {
		input    string
		expected string
	}{
		"keepLF": {
			input:    "line1\nline2\n",
			expected: "line1\nline2\n",
		},
		"convertCRLF": {
			input:    "line1\r\nline2\r\n",
			expected: "line1\nline2\n",
		},
		"convertCR": {
			input:    "line1\rline2\r",
			expected: "line1\nline2\n",
		},
		"mixedEndings": {
			input:    "line1\r\nline2\rline3\n",
			expected: "line1\nline2\nline3\n",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := NormalizeLineEndings(tc.input)
			if got != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}
