/*
Copyright (C) GRyCAP - I3M - UPV

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"encoding/hex"
	"testing"
)

// TestGenerateTokenLength checks if the generated token has the correct length
func TestGenerateTokenLength(t *testing.T) {
	token := GenerateToken()
	expectedLength := 64 // 32 bytes * 2 (hex encoding)
	if len(token) != expectedLength {
		t.Errorf("Expected token length of %d, but got %d", expectedLength, len(token))
	}
}

// TestGenerateTokenUniqueness checks if multiple generated tokens are unique
func TestGenerateTokenUniqueness(t *testing.T) {
	token1 := GenerateToken()
	token2 := GenerateToken()
	if token1 == token2 {
		t.Error("Expected tokens to be unique, but they are the same")
	}
}

// TestGenerateTokenHexEncoding checks if the generated token is a valid hex string
func TestGenerateTokenHexEncoding(t *testing.T) {
	token := GenerateToken()
	_, err := hex.DecodeString(token)
	if err != nil {
		t.Errorf("Expected a valid hex string, but got an error: %v", err)
	}
}
