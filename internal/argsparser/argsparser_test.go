package argsparser

import (
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input   string
		want    time.Duration
		wantErr bool
	}{
		{"1h", time.Hour, false},
		{"2d", 48 * time.Hour, false},
		{"3w", 504 * time.Hour, false},
		{"1.5h", 90 * time.Minute, false},
		{"1d2h", 26 * time.Hour, false},
		{"1w3d", (7 + 3) * 24 * time.Hour, false},
		{"1M", 30 * 24 * time.Hour, false},
		{"1y", 365 * 24 * time.Hour, false},
		{"-2h", -2 * time.Hour, false},
		{"", 0, false},
		{"5d2x", 0, true},
		{"1x", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseDuration(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDuration(%q) error = %v, wantErr %v, got %v", tt.input, err, tt.wantErr, got)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseDuration(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
