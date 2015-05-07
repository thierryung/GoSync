package main

// TODO: Add modulo to use smaller int and prevent buffer overflow
// TODO: Optimize algo, bit shifting instead of modulo?
// TODOING: Handle errors, especially from readFull (when we still have bytes but have reached the end)
// TODO: When reading, need to remember what is the current length in window (say we read less than len(window))
// TODONE: Defer all file closing
// TODO: Have currBlock and currByte share same underlying array for memory optimization
// TODO: Profile CPU & Memory usage
// TODONE: Variable names, decide case style
// TODO: Need file versioning, especially for conflict handling
// TODO: Make it a clean architecture
// Tests: Add/remove char in the beginning, middle, end, random place
// Tests: Add/remove 2 chars in the beginning, middle, end, random place
// Tests: Add/remove multiple chars in random places
// Tests: Unit tests!

import (
	"bufio"
	"crypto/md5"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"runtime/pprof"
	"time"
)

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
var memprofile = flag.String("memprofile", "", "write memory profile to this file")

// Represents our circling window
type WindowBytes struct {
	// The current bytes for this window
	currBytes []byte
	// Start index since this is a circling window
	startIndex int
	// Length of bytes used in the circling window, since we may read less than window
	length int
	// The current bytes for this block (meaning at least the size of currBytes)
	currBlock []byte
}

// Represents a specific block, during hashing
type BlockHash struct {
	length         int
	hash           [16]byte
	positionInFile int
}

// Convenient container with params for a file hash
type FileHashParam struct {
	filepath              string
	windowSize            int
	hash, primeRoot, mask uint64
}

// Represents a change between source and dest file
type FileChange struct {
	dataToAdd []byte
	// TODO: Check if int is enough (overflow)
	positionInSourceFile int
	// TODO: Check if int is enough (overflow)
	lengthToAdd int
	// TODO: Check if int is enough (overflow)
	lengthToRemove int
}

func (w *WindowBytes) init(windowSize int) {
	// Init our window byte
	w.currBytes = make([]byte, windowSize)
	// Init our block array at a larger size
	// It will automatically expand as needed, but having it large enough is better
	w.currBlock = make([]byte, windowSize*10000)
	w.startIndex = 0
}

func (w *WindowBytes) getFirstByte() byte {
	return w.currBytes[w.startIndex]
}

func (w *WindowBytes) getBytes() []byte {
	var windowSize = len(w.currBytes)
	var currBytes = make([]byte, windowSize)
	c := 0

	// Circling window. We start at wherever startIndex is
	// We keep going until we come back to startIndex, or we reached max length
	for i := w.startIndex; (i != w.startIndex || c == 0) && c < w.length; i++ {
		currBytes[c] = w.currBytes[i]
		c++
		if i >= windowSize-1 {
			i = -1
		}
	}
	return currBytes
}

func (w *WindowBytes) addByte(b byte) {
	// Add byte to window
	w.currBytes[w.startIndex] = b
	w.startIndex++
	if w.startIndex >= len(w.currBytes) {
		w.startIndex = 0
	}

	// Add byte to block
	w.currBlock = append(w.currBlock, b)
}

func (w *WindowBytes) readFull(reader *bufio.Reader) (n int, err error) {
	// Reset window bytes
	w.startIndex = 0
	var c int = 0
	var cmax int = len(w.currBytes)
	for c = 0; c < cmax; c++ {
		w.currBytes[c], err = reader.ReadByte()
		if err != nil {
			break
		}
	}
	w.length = c

	// Copy this window to our block
	copy(w.currBlock, w.currBytes)
	// Truncate off the rest
	w.currBlock = w.currBlock[0:w.length]

	return c, err
}

func iPow(a uint64, b int) uint64 {
	var result uint64 = 1

	for 0 != b {
		if 0 != (b & 1) {
			result *= uint64(a)

		}
		b >>= 1
		a *= a
	}

	return result
}

func calculateLengthBetween(b []BlockHash, start, end int) int {
	var result int = 0
	for i := start; i < end; i++ {
		result += b[i].length
	}
	return result
}

func hashFile(param FileHashParam) []BlockHash {
	var c, startWindowPosition, index, cmatch, lenMin, lenMax, lenCurr int = 0, 0, 0, 0, -1, -1, -1
	var under, _100, _200, _300, _400, _500, _plus int = 0, 0, 0, 0, 0, 0, 0
	var hash uint64 = 0
	var currByte byte
	var window WindowBytes
	var hashBlock [16]byte
	var arrBlockHash []BlockHash

	window.init(param.windowSize)

	// Read file
	f, err := os.Open(param.filepath)
	if err != nil {
		return arrBlockHash
	}
	defer func() {
		if err := f.Close(); err != nil {
			return
		}
	}()
	reader := bufio.NewReader(f)

	// Reset the read window, we'll slide from there
	lenCurr, err = window.readFull(reader)
	if err != nil {
		return arrBlockHash
	}
	c += lenCurr
	// Calculate window hash (first time)
	for index, currByte = range window.currBytes {
		hash += uint64(currByte) * iPow(param.primeRoot, param.windowSize-index-1)
	}

	for {
		// Check if we fit the match, and at least a certain amount of bytes
		if (hash | param.mask) == hash {
			if lenMax == -1 || lenCurr > lenMax {
				lenMax = lenCurr
			}
			if lenMin == -1 || lenCurr < lenMin {
				lenMin = lenCurr
			}

			if lenCurr < 50000 {
				under++
			} else if lenCurr < 100000 {
				_100++
			} else if lenCurr < 200000 {
				_200++
			} else if lenCurr < 300000 {
				_300++
			} else if lenCurr < 400000 {
				_400++
			} else if lenCurr < 600000 {
				_500++
			} else {
				_plus++
			}

			// New match, md5 it
			cmatch++
			hashBlock = md5.Sum(window.currBlock)
			arrBlockHash = append(arrBlockHash, BlockHash{length: lenCurr, hash: hashBlock, positionInFile: startWindowPosition})

			// Reset the read window, we'll slide from there
			lenCurr, err = window.readFull(reader)
			// TODO: Check error here? Since readFull can return error
			startWindowPosition = c
			c += lenCurr
			// Calculate next window hash
			for index, currByte = range window.currBytes {
				hash += uint64(currByte) * iPow(param.primeRoot, param.windowSize-index-1)
			}

		} else {
			// No fit, we keep going for this block
			currByte, err = reader.ReadByte()
			if err != nil {
				break
			}

			// Magic hash
			hash -= uint64(window.getFirstByte()) * iPow(param.primeRoot, param.windowSize-1)
			hash *= param.primeRoot
			hash += uint64(currByte)

			// Add new byte read
			window.addByte(currByte)
			c++
			lenCurr++
		}
	}

	// Last block, if not empty
	if lenCurr > 0 {
		hashBlock = md5.Sum(window.currBlock)
		arrBlockHash = append(arrBlockHash, BlockHash{length: lenCurr, hash: hashBlock, positionInFile: startWindowPosition})
	}

	fmt.Printf("Found %d matches!\n", cmatch)
	fmt.Printf("Went through %d bytes!\n", c)
	fmt.Printf("Min block %d bytes!\n", lenMin)
	fmt.Printf("Max block %d bytes!\n", lenMax)
	fmt.Printf("%d, %d, %d, %d, %d, %d, %d\n\n", under, _100, _200, _300, _400, _500, _plus)

	return arrBlockHash
}

// TODO: Handle logic when change is at the very beginning or at the very end
// TODO: Make a prettier loop (loop ending i++ j++)
// TODO: Free array hash map (or check memory for every init)
func compareFileHashes(arrHashSource, arrHashDest []BlockHash) []FileChange {
	var arrFileChange []FileChange
	var i, j int = 0, 0
	var lenSource, lenDest int = len(arrHashSource), len(arrHashDest)
	var bIsCheckingDiff bool = false
	var iHashPosSource, iHashPosDest int
	var mapHashSource, mapHashDest map[[16]byte]int

	// Loop through both arrays, find differences
	for i < lenSource && j < lenDest {
		// Logic for loop while currently checking a diff
		if bIsCheckingDiff {
			mapHashSource[arrHashSource[i].hash] = i
			mapHashDest[arrHashDest[j].hash] = j

			// Check if any past hash matches one of our current hash (see algo)
			iPosMatchSource, okSource := mapHashSource[arrHashDest[j].hash]
			iPosMatchDest, okDest := mapHashDest[arrHashSource[i].hash]
			// In which case, we're now simply exchanging data (add/remove)
			if okSource {
				arrFileChange = append(arrFileChange, FileChange{lengthToAdd: calculateLengthBetween(arrHashSource, iHashPosSource, iPosMatchSource), positionInSourceFile: arrHashSource[iHashPosSource].positionInFile, lengthToRemove: calculateLengthBetween(arrHashDest, iHashPosDest, j)})

				// We're done with checking diff, go back to standard loop
				i = iPosMatchSource
				bIsCheckingDiff = false

			} else if okDest {
				arrFileChange = append(arrFileChange, FileChange{lengthToAdd: calculateLengthBetween(arrHashSource, iHashPosSource, i), positionInSourceFile: arrHashSource[iHashPosSource].positionInFile, lengthToRemove: calculateLengthBetween(arrHashDest, iHashPosDest, iPosMatchDest)})

				// We're done with checking diff, go back to standard loop
				j = iPosMatchDest
				bIsCheckingDiff = false
			}

			// Go to next set
			i++
			j++
			continue
		}

		// Matching data, simply go to next
		if arrHashSource[i].hash == arrHashDest[j].hash {
			i++
			j++

		} else {
			// Non matching data, remember current state
			bIsCheckingDiff = true
			iHashPosSource = i
			iHashPosDest = j
			// Initialize our map array
			mapHashSource = make(map[[16]byte]int)
			mapHashSource[arrHashSource[i].hash] = i
			mapHashDest = make(map[[16]byte]int)
			mapHashDest[arrHashDest[j].hash] = j
			// And go to next
			i++
			j++
		}
	}

	// We're now out of the loop, was the last block different?
	// Or do we have a hash list longer than the other?
	if bIsCheckingDiff || lenSource != lenDest {
		// In this case, we're simply overriding data (removing old, adding new)
		arrFileChange = append(arrFileChange, FileChange{lengthToAdd: calculateLengthBetween(arrHashSource, i-1, len(arrHashSource)), positionInSourceFile: arrHashSource[i-1].positionInFile, lengthToRemove: calculateLengthBetween(arrHashDest, j-1, len(arrHashDest))})
	}

	return arrFileChange
}

func updateDeltaData(arrFileChange []FileChange, fileHashParamSource FileHashParam) {
	f, err := os.Open(fileHashParamSource.filepath)
	if err != nil {
		return
	}
	defer func() {
		if err := f.Close(); err != nil {
			return
		}
	}()
	// Loop through our changes
	for key := range arrFileChange {
		if arrFileChange[key].lengthToAdd <= 0 {
			continue
		}
		// TODO: Better error check, check for offset ok
		_, err := f.Seek(int64(arrFileChange[key].positionInSourceFile), 0)
		if err != nil {
			return
		}
		newData := make([]byte, arrFileChange[key].lengthToAdd)
		// TODO: Better error check, check for num read ok
		_, err = io.ReadFull(f, newData)
		if err != nil {
			return
		}
		//fmt.Printf("Read from source pos %d: %s\n", arrFileChange[key].positionInSourceFile, newData)
		arrFileChange[key].dataToAdd = newData
	}
}

// 1. Final data will be created in a temp file
// 2. Original file will be moved
// 3. Temp file renamed to original
// 4. Do some file integrity checking
// 5. If all goes well, remove original
func updateDestinationFile(arrFileChange []FileChange, fileHashParamDest FileHashParam) {
	// TODO: Check if we need to manually split chunks of data read

	var iToRead, iLastFilePointerPosition int = 0, 0

	// open input file
	fi, err := os.Open(fileHashParamDest.filepath)
	if err != nil {
		return
	}
	// close fi on exit and check for its returned error
	defer func() {
		if err := fi.Close(); err != nil {
			return
		}
	}()
	// make a read buffer
	r := bufio.NewReader(fi)

	// open output file
	fo, err := os.Create(fileHashParamDest.filepath + ".tmp")
	if err != nil {
		return
	}
	// close fo on exit and check for its returned error
	defer func() {
		if err := fo.Close(); err != nil {
			return
		}
	}()
	// make a write buffer
	w := bufio.NewWriter(fo)

	// Loop through our changes
	for _, fileChange := range arrFileChange {
		// Read until change position
		iToRead = fileChange.positionInSourceFile - iLastFilePointerPosition
		buf := make([]byte, iToRead)
		// read a chunk
		// TODO: Check errors
		n, err := r.Read(buf)
		if err != nil {
			break
		}
		// Write data up until position of change
		if _, err := w.Write(buf[:n]); err != nil {
			break
		}

		// Process the add
		if _, err := w.Write(fileChange.dataToAdd); err != nil {
			return
		}

		// Process the remove (skip next x bytes)
		// For now we're "reading" bytes to move file pointer, as Seek is not supported by bufio
		buf = make([]byte, fileChange.lengthToRemove)
		_, err = r.Read(buf)
		if err != nil {
			return
		}

		// Update our file pointer position
		iLastFilePointerPosition = fileChange.positionInSourceFile + fileChange.lengthToAdd
	}

	// Write last chunk
	buf := make([]byte, 1024)
	for {
		// read a chunk
		n, err := r.Read(buf)
		if err != nil && err != io.EOF {
			break
		}
		if n == 0 {
			break
		}

		// write a chunk
		if _, err := w.Write(buf[:n]); err != nil {
			break
		}
	}

	if err = w.Flush(); err != nil {
		return
	}
}

func main() {

	flag.Parse()
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	start := time.Now()
	var primeRoot uint64 = 31

	// var windowSize = 1024
	// var mask uint64 = (1 << 19) - 1
	// var strFilepath1 string = "/home/thierry/projects/vol1"
	// var strFilepath2 string = "/home/thierry/projects/vol2"

	var windowSize = 3
	var mask uint64 = (1 << 2) - 1
	var strFilepath1 string = "/home/thierry/projects/vol.test"
	var strFilepath2 string = "/home/thierry/projects/vol2.test"

	var arrBlockHash []BlockHash
	var fileHashParam FileHashParam
	var arrBlockHash2 []BlockHash
	var fileHashParam2 FileHashParam
	var arrFileChange []FileChange

	// Hash file 1
	fileHashParam = FileHashParam{filepath: strFilepath1, windowSize: windowSize, primeRoot: primeRoot, mask: mask}
	arrBlockHash = hashFile(fileHashParam)

	// Hash file 2
	fileHashParam2 = FileHashParam{filepath: strFilepath2, windowSize: windowSize, primeRoot: primeRoot, mask: mask}
	arrBlockHash2 = hashFile(fileHashParam2)

	// Compare two files
	arrFileChange = compareFileHashes(arrBlockHash, arrBlockHash2)
	//fmt.Println(arrFileChange)
	fmt.Printf("We found %d changes!\n", len(arrFileChange))

	// Get difference data
	updateDeltaData(arrFileChange, fileHashParam)

	// Update destination file
	updateDestinationFile(arrFileChange, fileHashParam2)

	elapsed := time.Since(start)
	fmt.Printf("Binomial took %s\n", elapsed)

	if *memprofile != "" {
		f1, err := os.Create(*memprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.WriteHeapProfile(f1)
		f1.Close()
	}
}
