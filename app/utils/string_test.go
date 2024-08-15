package utils

import (
	"testing"
)

func TestGetCodeFromUrl(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		expected    string
		expectPanic bool
	}{
		{
			name:     "Valid URL with code parameter",
			url:      "https://example.com/callback?code=abc123",
			expected: "abc123",
		},
		{
			name:        "URL without code parameter",
			url:         "https://example.com/callback?token=xyz456",
			expected:    "",
			expectPanic: true, // since codeString[5:] will panic due to index out of range
		},
		{
			name:        "URL with code parameter but no value",
			url:         "https://example.com/callback?code=",
			expected:    "",
			expectPanic: true, // since codeString[5:] will panic due to index out of range
		},
		{
			name:     "URL with code parameter at end",
			url:      "https://example.com/callback?other=123&code=xyz789",
			expected: "xyz789",
		},
		{
			name:        "Case sensitivity (code parameter in uppercase)",
			url:         "https://example.com/callback?Code=abc123",
			expected:    "",
			expectPanic: true, // since the regex won't match "Code", codeString will be empty leading to panic
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					if !tt.expectPanic {
						t.Errorf("TestGetCodeFromUrl() panicked when it shouldn't have")
					}
				}
			}()

			got := GetCodeFromUrl(tt.url)
			if got != tt.expected {
				t.Errorf("GetCodeFromUrl() = %v, want %v", got, tt.expected)
			}
		})
	}
}
