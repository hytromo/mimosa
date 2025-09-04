package hasher

import (
	"encoding/hex"
	"math"
	"sort"
	"sync"

	"log/slog"

	"github.com/hytromo/mimosa/internal/logger"
	"github.com/kalafut/imohash"
)

// HashFiles computes a hash of all files in the provided list
// and returns a single hash representing the unique state of all files.
// It produces the same hash for the same files, regardless of the order of the files.
func HashFiles(filePaths []string, nWorkers int) string {
	if len(filePaths) == 0 {
		return ""
	}

	fileChan := make(chan string, len(filePaths))
	hashChan := make(chan []byte, len(filePaths))
	finalWorkerCount := int(math.Max(1, float64(nWorkers)))
	workerCountChan := make(chan struct {
		workerID int
		count    int
	}, finalWorkerCount)
	var wg sync.WaitGroup

	// Worker function
	worker := func(id int) {
		defer wg.Done()
		count := 0
		for path := range fileChan {
			hash, err := imohash.SumFile(path)
			if err == nil {
				hashChan <- hash[:]
				count++
			} else {
				slog.Debug("Error hashing file", "path", path, "error", err)
			}
		}
		workerCountChan <- struct {
			workerID int
			count    int
		}{id, count}
	}

	wg.Add(finalWorkerCount)
	for i := 0; i < finalWorkerCount; i++ {
		go worker(i)
	}

	for _, path := range filePaths {
		fileChan <- path
	}
	close(fileChan)

	wg.Wait()
	close(hashChan)
	close(workerCountChan)

	// Collect all hashes
	var fileHashes [][]byte
	for h := range hashChan {
		fileHashes = append(fileHashes, h)
	}

	// Collect worker stats
	workerStats := make([]int, finalWorkerCount)
	for stat := range workerCountChan {
		workerStats[stat.workerID] = stat.count
	}

	if logger.IsDebugEnabled() {
		// Print the number of files and their paths
		slog.Debug("Deducting file hash from files", "count", len(filePaths))
		for _, path := range filePaths {
			slog.Debug("File path", "path", path)
		}
		slog.Debug("Files hashed per worker", "workers", nWorkers)
		for i, c := range workerStats {
			if c > 0 {
				slog.Debug("Worker stats", "worker", i, "count", c)
			}
		}
	}

	// Sort the hashes to ensure consistent order
	sort.Slice(fileHashes, func(i, j int) bool {
		return string(fileHashes[i]) < string(fileHashes[j])
	})

	// Concatenate all hashes and hash the result for a final hash
	joined := joinHashes(fileHashes)
	finalHash := imohash.Sum(joined)
	return hex.EncodeToString(finalHash[:])
}

func joinHashes(hashes [][]byte) []byte {
	var out []byte
	for _, h := range hashes {
		out = append(out, h...)
	}
	return out
}
