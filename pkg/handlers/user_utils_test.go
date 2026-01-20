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

package handlers

import (
	"reflect"
	"testing"
)

func TestSanitizeUsers(t *testing.T) {
	tests := []struct {
		name     string
		users    []string
		expected []string
	}{
		{
			name:     "normal users",
			users:    []string{"user1", "user2", "user3"},
			expected: []string{"user1", "user2", "user3"},
		},
		{
			name:     "users with whitespace",
			users:    []string{" user1 ", "user2", " user3 "},
			expected: []string{"user1", "user2", "user3"},
		},
		{
			name:     "users with tabs and newlines",
			users:    []string{"user1\t", "\nuser2", "  user3  "},
			expected: []string{"user1", "user2", "user3"},
		},
		{
			name:     "empty strings mixed with valid users",
			users:    []string{"user1", "", "  ", "user2", "\t", "user3"},
			expected: []string{"user1", "user2", "user3"},
		},
		{
			name:     "only whitespace users",
			users:    []string{"  ", "\t", "\n", "   "},
			expected: []string{},
		},
		{
			name:     "empty input slice",
			users:    []string{},
			expected: []string{},
		},
		{
			name:     "nil input slice",
			users:    nil,
			expected: []string{},
		},
		{
			name:     "users with special characters",
			users:    []string{"user-one", "user_two", "user.three", "user four"},
			expected: []string{"user-one", "user_two", "user.three", "user four"},
		},
		{
			name:     "mixed case users",
			users:    []string{"User1", "USER2", "uSeR3"},
			expected: []string{"User1", "USER2", "uSeR3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeUsers(tt.users)

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestSanitizeUsersEdgeCases(t *testing.T) {
	t.Run("users with multiple spaces", func(t *testing.T) {
		users := []string{"  user  with  multiple  spaces  "}
		expected := []string{"user  with  multiple  spaces"}

		result := sanitizeUsers(users)

		if len(result) != 1 {
			t.Errorf("Expected 1 user, got %d", len(result))
		}

		if result[0] != expected[0] {
			t.Errorf("Expected '%s', got '%s'", expected[0], result[0])
		}
	})

	t.Run("users with unicode whitespace", func(t *testing.T) {
		users := []string{"user\u00A0", "user\u2009", "user\u3000"}
		expected := []string{"user", "user", "user"}

		result := sanitizeUsers(users)

		if !reflect.DeepEqual(result, expected) {
			t.Errorf("Expected %v, got %v", expected, result)
		}
	})

	t.Run("large input slice", func(t *testing.T) {
		// Test with many users to ensure performance is reasonable
		users := make([]string, 1000)
		expected := make([]string, 1000)

		for i := 0; i < 1000; i++ {
			users[i] = "  user" + string(rune(i%26+65)) + "  "
			expected[i] = "user" + string(rune(i%26+65))
		}

		result := sanitizeUsers(users)

		if !reflect.DeepEqual(result, expected) {
			t.Errorf("Large slice test failed")
		}
	})
}

func TestSanitizeUsersDuplicateHandling(t *testing.T) {
	t.Run("duplicates are preserved", func(t *testing.T) {
		users := []string{"user1", "user1", "user2", "user1", "user2", "user2"}
		expected := []string{"user1", "user1", "user2", "user1", "user2", "user2"}

		result := sanitizeUsers(users)

		if !reflect.DeepEqual(result, expected) {
			t.Errorf("Expected duplicates to be preserved, got %v", result)
		}
	})

	t.Run("duplicates with whitespace", func(t *testing.T) {
		users := []string{" user1 ", "user1", "  user1  ", "user2"}
		expected := []string{"user1", "user1", "user1", "user2"}

		result := sanitizeUsers(users)

		if !reflect.DeepEqual(result, expected) {
			t.Errorf("Expected %v, got %v", expected, result)
		}
	})
}

func TestSanitizeUsersPreserveOrder(t *testing.T) {
	t.Run("order is preserved", func(t *testing.T) {
		users := []string{"zeta", "alpha", "beta", "gamma", "delta"}
		expected := []string{"zeta", "alpha", "beta", "gamma", "delta"}

		result := sanitizeUsers(users)

		if !reflect.DeepEqual(result, expected) {
			t.Errorf("Expected order to be preserved, got %v", result)
		}
	})

	t.Run("order with empty strings", func(t *testing.T) {
		users := []string{"first", "", "second", "  ", "third", "\t", "fourth"}
		expected := []string{"first", "second", "third", "fourth"}

		result := sanitizeUsers(users)

		if !reflect.DeepEqual(result, expected) {
			t.Errorf("Expected %v, got %v", expected, result)
		}
	})
}

func TestSanitizeUsersMemoryAllocation(t *testing.T) {
	// Test that the function properly allocates memory for the expected output size
	t.Run("memory allocation", func(t *testing.T) {
		users := []string{"user1", "user2", "user3"}

		result := sanitizeUsers(users)

		// Check that capacity is correct (should be same as input length minus empty strings)
		expectedLen := 3
		if len(result) != expectedLen {
			t.Errorf("Expected length %d, got %d", expectedLen, len(result))
		}

		// Check that capacity is not excessively larger than length
		if cap(result) > expectedLen*2 {
			t.Errorf("Expected capacity to be reasonable, got %d", cap(result))
		}
	})
}
