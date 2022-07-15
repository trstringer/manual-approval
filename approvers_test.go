package main

import (
	"reflect"
	"testing"
)

func TestDeduplicateUsers(t *testing.T) {
	testCases := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "with_duplicate_user",
			input:    []string{"first", "second", "first"},
			expected: []string{"first", "second"},
		},
		{
			name:     "without_duplicate_user",
			input:    []string{"first", "second"},
			expected: []string{"first", "second"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			actual := deduplicateUsers(testCase.input)
			if !reflect.DeepEqual(testCase.expected, actual) {
				t.Fatalf(
					"unequal depulicated: expected %v actual %v",
					testCase.expected,
					actual,
				)
			}
		})
	}
}
