package hasher

import (
	"encoding/hex"
	"math"
	"runtime"
	"sort"
	"sync"

	"github.com/kalafut/imohash"
)

// HashFiles computes a hash of all files in the provided list
// and returns a single hash representing the unique state of all files.
func HashFiles(filePaths []string) (string, error) {
	if len(filePaths) == 0 {
		return "", nil
	}

	// as many workers as files, up to num of CPUs-1
	nWorkers := int(math.Min(float64(len(filePaths)), math.Max(float64(runtime.NumCPU()-1), 1)))

	fileChan := make(chan string, len(filePaths))
	hashChan := make(chan []byte, len(filePaths))
	workerCountChan := make(chan struct {
		workerID int
		count    int
	}, nWorkers)
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
			}
		}
		workerCountChan <- struct {
			workerID int
			count    int
		}{id, count}
	}

	wg.Add(nWorkers)
	for i := 0; i < nWorkers; i++ {
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
	workerStats := make([]int, nWorkers)
	for stat := range workerCountChan {
		workerStats[stat.workerID] = stat.count
	}

	// if log.IsLevelEnabled(log.DebugLevel) {
	// 	// Print the number of files and their paths
	// 	log.Debugf("Deducting file hash from %d files:", len(filePaths))
	// 	for _, path := range filePaths {
	// 		log.Debugln(path)
	// 	}
	// 	log.Debugf("Files hashed per worker (%v total workers):", nWorkers)
	// 	for i, c := range workerStats {
	// 		if c > 0 {
	// 			log.Debugf("  Worker %d: %d files", i, c)
	// 		}
	// 	}
	// }

	// Sort the hashes to ensure consistent order
	sort.Slice(fileHashes, func(i, j int) bool {
		return string(fileHashes[i]) < string(fileHashes[j])
	})

	// Concatenate all hashes and hash the result for a final hash
	joined := joinHashes(fileHashes)
	finalHash := imohash.Sum(joined)
	return hex.EncodeToString(finalHash[:]), nil
}

func joinHashes(hashes [][]byte) []byte {
	var out []byte
	for _, h := range hashes {
		out = append(out, h...)
	}
	return out
}
