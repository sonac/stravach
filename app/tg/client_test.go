package tg

import (
	"regexp"
	"strings"
	"testing"
)

// TestCleanName tests the cleanName method of the Telegram struct.
func TestCleanName(t *testing.T) {
	tgInstance := &Telegram{} // cleanName doesn't depend on other fields of Telegram struct

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple name",
			input: "My Awesome Ride",
			want:  "My Awesome Ride",
		},
		{
			name:  "name with leading/trailing spaces",
			input: "  My Awesome Ride  ",
			want:  "My Awesome Ride",
		},
		{
			name:  "name with special characters",
			input: "My Ride!@#$%^&*()_+-={}|[]\\:\";'<>?,./",
			want:  "My Ride!&()_-\"'?,.",
		},
		{
			name:  "name with multiple spaces between words",
			input: "My    Awesome   Ride",
			want:  "My Awesome Ride",
		},
		{
			name:  "name with numbers",
			input: "Ride 123",
			want:  "Ride 123",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "only special characters",
			input: "!@#$%^&*()",
			want:  "!&()",
		},
		{
			name:  "name with hyphens and underscores",
			input: "My_Activity-Ride",
			want:  "My_Activity-Ride",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tgInstance.cleanName(tt.input)
			if got != tt.want {
				t.Errorf("Telegram.cleanName() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Helper function to manually apply the regex logic for verification of test cases,
// as the original cleanName function is unexported if this test were in tg_test package.
// Since it's in 'tg' package, we can directly call tgInstance.cleanName.
// This helper is more for documenting the expected behavior based on the regex.
func expectedCleanNameByRegex(name string) string {
	name = strings.TrimSpace(name)
	re := regexp.MustCompile(`[^a-zA-Z0-9\s\-_.,!?'"()&]+`)
	name = re.ReplaceAllString(name, "")
	re = regexp.MustCompile(`\s+`)
	name = re.ReplaceAllString(name, " ")
	return name
}
