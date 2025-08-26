package hasher

import (
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/hytromo/mimosa/internal/configuration"
	"github.com/hytromo/mimosa/internal/utils/fileutil"
	log "github.com/sirupsen/logrus"
)

// DockerBuildCommand is a struct that contains the information needed to hash a docker build command
type DockerBuildCommand struct {
	DockerfilePath        string
	DockerignorePath      string
	BuildContexts         map[string]string
	AllRegistryDomains    []string
	CmdWithTagPlaceholder []string
}

func registryDomainsHash(registryDomains []string) string {
	slices.Sort(registryDomains)
	return HashStrings(registryDomains)
}

func HashBuildCommand(command DockerBuildCommand) string {
	registryDomainsHash := registryDomainsHash(command.AllRegistryDomains)

	allLocalContexts := map[string]string{} // context name -> context path
	// find all the included files of the build contexts that are local (not https://, not docker-image://, not oci-layout://)
	for _, context := range command.BuildContexts {
		parts := strings.Split(context, "=")
		if len(parts) != 2 {
			continue
		}
		contextName := parts[0]
		contextPath := parts[1]
		if !strings.HasPrefix(contextPath, "https://") && !strings.HasPrefix(contextPath, "docker-image://") && !strings.HasPrefix(contextPath, "oci-layout://") {
			allLocalContexts[contextName] = contextPath
		}
	}

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

				if contextName == configuration.MainBuildContextName {
					// need to include dockerfile and dockerignore in the to-be-hashed files
					includedFiles = append(includedFiles, command.DockerfilePath)
					if command.DockerignorePath != "" {
						includedFiles = append(includedFiles, command.DockerignorePath)
					}
				}

				if err != nil {
					log.Errorf("Error getting included files for context %s: %v", contextName, err)
					includedFilesChan <- []string{}
					continue
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

	if log.IsLevelEnabled(log.DebugLevel) {
		log.Debugf("Hashing %d files across %d build contexts", len(allFilesAcrossContexts), len(allLocalContexts))
	}

	return HashStrings([]string{
		// the command itself with placeholdered tags
		strings.Join(command.CmdWithTagPlaceholder, " "),
		// the domains used to push the image to
		// including this is important for the edge case where the same
		// exact build is repeated with different domains - promotion doesn't work then
		registryDomainsHash,
		// includes all the build contexts' files, plus dockerfile (and dockerignore optionally)
		HashFiles(allFilesAcrossContexts, nWorkers),
	})
}
