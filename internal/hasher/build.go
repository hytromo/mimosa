package hasher

import (
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/hytromo/mimosa/internal/configuration"
	"github.com/hytromo/mimosa/internal/logger"
	"github.com/hytromo/mimosa/internal/utils/fileutil"
	"github.com/samber/lo"
)

// DockerBuildCommand is a struct that contains the information needed to hash a docker build command
type DockerBuildCommand struct {
	DockerfilePath         string
	DockerignorePath       string
	BuildContexts          map[string]string
	AllRegistryDomains     []string
	CmdWithoutTagArguments []string
}

func registryDomainsHash(registryDomains []string) string {
	// Create a copy to avoid modifying the original slice
	domains := make([]string, len(registryDomains))
	copy(domains, registryDomains)

	// Remove duplicates and sort
	domains = lo.Uniq(domains)
	slices.Sort(domains)

	return HashStrings(domains)
}

func HashBuildCommand(command DockerBuildCommand) string {
	registryDomainsHash := registryDomainsHash(command.AllRegistryDomains)

	allLocalContexts := map[string]string{} // context name -> context path
	// find all the included files of the build contexts that are local (not https://, not docker-image://, not oci-layout://)
	for contextName, contextPath := range command.BuildContexts {
		if !strings.HasPrefix(contextPath, "https://") && !strings.HasPrefix(contextPath, "docker-image://") && !strings.HasPrefix(contextPath, "oci-layout://") {
			allLocalContexts[contextName] = contextPath
		}
	}

	slog.Debug("All local contexts", "contexts", allLocalContexts)

	// up to num of CPUs-1
	nWorkers := int(math.Max(float64(runtime.NumCPU()-1), 1))

	// Create channels for the worker pool
	dockerContextChan := make(chan struct {
		contextName string
		contextPath string
	}, len(allLocalContexts))
	includedFilesChan := make(chan []string, len(allLocalContexts))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < nWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for context := range dockerContextChan {
				contextName := context.contextName
				contextPath := context.contextPath

				// get the dockerignore path for this context
				// if we are in the main context we have the default .dockerignore resolution, otherwise we expect a .dockerignore file in the root of the context path
				dockerIgnorePath := command.DockerignorePath
				if contextName != configuration.MainBuildContextName {
					dockerIgnorePath = filepath.Join(contextPath, ".dockerignore")
					// check if file exists:
					if _, err := os.Stat(dockerIgnorePath); os.IsNotExist(err) {
						dockerIgnorePath = ""
					}
				}

				// Get all included files for this context
				includedFiles, err := fileutil.IncludedFiles(contextPath, dockerIgnorePath)

				if err != nil {
					slog.Error("Error getting included files for context", "context", contextName, "error", err)
					includedFilesChan <- []string{}
					continue
				}

				if contextName == configuration.MainBuildContextName {
					// need to include dockerfile and dockerignore in the to-be-hashed files
					dockerfileAbsolutePath, err := filepath.Abs(command.DockerfilePath)
					if err != nil {
						slog.Error("Error getting absolute path for dockerfile", "error", err)
					} else {
						includedFiles = append(includedFiles, dockerfileAbsolutePath)
					}
					if command.DockerignorePath != "" {
						dockerIgnoreAbsolutePath, err := filepath.Abs(command.DockerignorePath)
						if err != nil {
							slog.Error("Error getting absolute path for dockerignore", "error", err)
						} else {
							includedFiles = append(includedFiles, dockerIgnoreAbsolutePath)
						}
					}
				}

				// Hash the context files
				includedFilesChan <- includedFiles
			}
		}()
	}

	// Send all contexts to workers
	for contextName, contextPath := range allLocalContexts {
		dockerContextChan <- struct {
			contextName string
			contextPath string
		}{contextName, contextPath}
	}
	close(dockerContextChan)

	// Wait for all workers to complete
	wg.Wait()
	close(includedFilesChan)

	// Collect results
	allFilesAcrossContexts := make([]string, 0, len(allLocalContexts))
	for files := range includedFilesChan {
		allFilesAcrossContexts = append(allFilesAcrossContexts, files...)
	}

	if logger.IsDebugEnabled() {
		slog.Debug("Hashing files across build contexts", "fileCount", len(allFilesAcrossContexts), "contextCount", len(allLocalContexts))
	}

	return HashStrings([]string{
		// the command itself (without tags)
		strings.Join(command.CmdWithoutTagArguments, " "),
		// the domains used to push the image to
		// including this is important for the edge case where the same
		// exact build is repeated with different domains - promotion doesn't work then
		registryDomainsHash,
		// includes all the build contexts' files, plus dockerfile (and dockerignore optionally)
		HashFiles(allFilesAcrossContexts, nWorkers),
	})
}
