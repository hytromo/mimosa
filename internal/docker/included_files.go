package docker

import (
	"bytes"
	"os"
	"path/filepath"

	"github.com/moby/patternmatcher"
	"github.com/moby/patternmatcher/ignorefile"

	log "github.com/sirupsen/logrus"
)

func IncludedFiles(contextDir string, dockerignorePath string) ([]string, error) {
	var includedFiles []string

	if dockerignorePath == "" {
		// No .dockerignore: return all files recursively
		err := filepath.WalkDir(contextDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if path == contextDir {
				return nil
			}
			if d.IsDir() {
				return nil
			}
			absPath, err := filepath.Abs(path)
			if err != nil {
				return err
			}
			includedFiles = append(includedFiles, absPath)
			return nil
		})
		if err != nil {
			log.Debugln(err)
			return includedFiles, err
		}
		return includedFiles, nil
	}

	dockerignoreContent, err := os.ReadFile(dockerignorePath)
	if err != nil {
		log.Debugln(err)
		return includedFiles, err
	}

	// Parse patterns
	patterns, err := ignorefile.ReadAll(bytes.NewReader(dockerignoreContent))
	if err != nil {
		log.Debugln(err)
		return includedFiles, err
	}

	// Compile matcher
	pm, err := patternmatcher.New(patterns)
	if err != nil {
		log.Debugln(err)
		return includedFiles, err
	}

	err = filepath.WalkDir(contextDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(contextDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		excluded, _, err := pm.MatchesUsingParentResults(rel, patternmatcher.MatchInfo{})
		if err != nil {
			return err
		}
		if !excluded && !d.IsDir() {
			absPath, err := filepath.Abs(path)
			if err != nil {
				return err
			}
			includedFiles = append(includedFiles, absPath)
		}
		return nil
	})
	if err != nil {
		log.Debugln(err)
		return includedFiles, err
	}
	return includedFiles, nil
}
