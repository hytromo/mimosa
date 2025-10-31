package actions

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Duration
		hasError bool
	}{
		{
			name:     "simple hours",
			input:    "2h",
			expected: 2 * time.Hour,
			hasError: false,
		},
		{
			name:     "simple minutes",
			input:    "30m",
			expected: 30 * time.Minute,
			hasError: false,
		},
		{
			name:     "simple seconds",
			input:    "45s",
			expected: 45 * time.Second,
			hasError: false,
		},
		{
			name:     "days",
			input:    "3d",
			expected: 3 * 24 * time.Hour,
			hasError: false,
		},
		{
			name:     "weeks",
			input:    "2w",
			expected: 2 * 7 * 24 * time.Hour,
			hasError: false,
		},
		{
			name:     "months",
			input:    "1M",
			expected: 1 * 30 * 24 * time.Hour,
			hasError: false,
		},
		{
			name:     "years",
			input:    "1y",
			expected: 1 * 365 * 24 * time.Hour,
			hasError: false,
		},
		{
			name:     "mixed units",
			input:    "1d2h30m",
			expected: 1*24*time.Hour + 2*time.Hour + 30*time.Minute,
			hasError: false,
		},
		{
			name:     "decimal hours",
			input:    "1.5h",
			expected: 90 * time.Minute,
			hasError: false,
		},
		{
			name:     "negative duration",
			input:    "-2h",
			expected: -2 * time.Hour,
			hasError: false,
		},
		{
			name:     "zero duration",
			input:    "0s",
			expected: 0,
			hasError: false,
		},
		{
			name:     "invalid format",
			input:    "invalid-duration",
			expected: 0,
			hasError: false, // parseDuration returns 0 for invalid input without error
		},
		{
			name:     "empty string",
			input:    "",
			expected: 0,
			hasError: false,
		},
		{
			name:     "case insensitive days",
			input:    "2D",
			expected: 2 * 24 * time.Hour,
			hasError: false,
		},
		{
			name:     "case insensitive weeks",
			input:    "1W",
			expected: 1 * 7 * 24 * time.Hour,
			hasError: false,
		},
		{
			name:     "case insensitive years",
			input:    "1Y",
			expected: 1 * 365 * 24 * time.Hour,
			hasError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseDuration(tt.input)
			if tt.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestParseDurationShouldValidateInput(t *testing.T) {
	validInputs := []string{"1h", "30m", "2d", "1w"}
	for _, input := range validInputs {
		t.Run("valid_"+input, func(t *testing.T) {
			result, err := parseDuration(input)
			assert.NoError(t, err, "Valid input should not return error: %s", input)
			assert.NotEqual(t, time.Duration(0), result, "Valid input should not return 0 duration: %s", input)
		})
	}
}
