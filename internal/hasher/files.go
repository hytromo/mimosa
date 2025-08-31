package hasher

import (
	"encoding/hex"
	"math"
	"sort"
	"sync"

	"github.com/kalafut/imohash"
	log "github.com/sirupsen/logrus"
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
				log.Debugf("Error hashing file %s: %v", path, err)
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

	if log.IsLevelEnabled(log.DebugLevel) {
		// Print the number of files and their paths
		log.Debugf("Deducting file hash from %d files:", len(filePaths))
		for _, path := range filePaths {
			log.Debugln(path)
		}
		log.Debugf("Files hashed per worker (%v total workers):", nWorkers)
		for i, c := range workerStats {
			if c > 0 {
				log.Debugf("  Worker %d: %d files", i, c)
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
