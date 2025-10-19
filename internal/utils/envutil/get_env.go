package envutil

import (
	"os"
)

func GetEnv(key, defaultValue string) string {
	envValue := os.Getenv(key)

	if envValue == "" {
		return defaultValue
	}

	return envValue
}
