package docker

import "testing"

func TestIsACRRegistry(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		want     bool
	}{
		{"public cloud", "myregistry.azurecr.io", true},
		{"china cloud", "myregistry.azurecr.cn", true},
		{"germany cloud", "myregistry.azurecr.de", true},
		{"us gov cloud", "myregistry.azurecr.us", true},
		{"microsoft container registry", "mcr.microsoft.com", true},
		{"non-acr tld", "myregistry.azurecr.me", false},
		{"substring suffix attack", "evil.azurecr.io.attacker.com", false},
		{"unrelated host", "docker.io", false},
		{"substring prefix attack", "azurecr.io.evil.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isACRRegistry(tt.hostname); got != tt.want {
				t.Errorf("isACRRegistry(%q) = %v, want %v", tt.hostname, got, tt.want)
			}
		})
	}
}
