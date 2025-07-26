package fileutil

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// SaveJSON saves a struct as pretty-formatted JSON data to a specific path; it can optionally compress the final result
func SaveJSON(path string, dataToWrite interface{}) error {
	bytesToWrite, _ := json.MarshalIndent(dataToWrite, "", "\t")

	return os.WriteFile(path, bytesToWrite, 0644)
}

// DeleteEmptyDirectories deletes all empty directories in the specified path recursively (excluding the root directory).
func DeleteEmptyDirectories(path string) error {
	return deleteEmptyDirsHelper(path, path)
}

// deleteEmptyDirsHelper is a recursive helper function that deletes empty directories and ensures the root directory is not deleted.
func deleteEmptyDirsHelper(root, current string) error {
	entries, err := os.ReadDir(current)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			subdir := filepath.Join(current, entry.Name())
			if err := deleteEmptyDirsHelper(root, subdir); err != nil {
				return err
			}
		}
	}

	entries, err = os.ReadDir(current)
	if err != nil {
		return err
	}

	if len(entries) == 0 && current != root {
		if err := os.Remove(current); err != nil {
			return err
		}
	}

	return nil
}
