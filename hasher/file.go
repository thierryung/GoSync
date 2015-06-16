package hasher

import (
	"bufio"
	"crypto/md5"
	"fmt"
	"os"

	"thierry/sync/math"
)

// Represents a specific block, during hashing
type BlockHash struct {
	Length         int
	Hash           [16]byte
	PositionInFile int
}

// TODO: Possible optimization, the result of Pow could be cached since we always use the same #
func HashFile(strFilepath string) []BlockHash {
	var c, startWindowPosition, index, cmatch, lenCurr int = 0, 0, 0, 0, -1
	var hash uint64 = 0
	var currByte byte
	var window WindowBytes
	var hashBlock [16]byte
	var arrBlockHash []BlockHash

	//
	fmt.Println("Start hash of file ", strFilepath)

	// Check if file exists
	if _, err := os.Stat(strFilepath); os.IsNotExist(err) {
		return arrBlockHash
	}

	window.init(HASH_WINDOW_SIZE)

	// Read file
	f, err := os.Open(strFilepath)
	if err != nil {
		fmt.Println("Err in opening file", err)
		return arrBlockHash
	}
	defer func() {
		if err := f.Close(); err != nil {
			fmt.Println("Err in closing file", err)
			return
		}
	}()
	reader := bufio.NewReader(f)

	// Reset the read window, we'll slide from there
	lenCurr, err = window.readFull(reader)
	if err != nil && lenCurr <= 0 {
		fmt.Println("Err in reading file", err)
		return arrBlockHash
	}
	c += lenCurr
	// Calculate window hash (first time)
	for index, currByte = range window.currBytes {
		hash += uint64(currByte) * math.Pow(HASH_PRIME_ROOT, HASH_WINDOW_SIZE-index-1)
	}

	for {
		// Check if we fit the match, and at least a certain amount of bytes
		if (hash | HASH_MASK) == hash {

			// New match, md5 it
			cmatch++
			hashBlock = md5.Sum(window.currBlock)
			arrBlockHash = append(arrBlockHash, BlockHash{Length: lenCurr, Hash: hashBlock, PositionInFile: startWindowPosition})

			// Reset the read window, we'll slide from there
			lenCurr, err = window.readFull(reader)
			if err != nil && lenCurr <= 0 {
				fmt.Println("Error in hashfile", err)
				break
			}
			startWindowPosition = c
			c += lenCurr
			// Calculate next window hash
			hash = 0
			for index, currByte = range window.currBytes {
				hash += uint64(currByte) * math.Pow(HASH_PRIME_ROOT, HASH_WINDOW_SIZE-index-1)
			}

		} else {
			// No fit, we keep going for this block
			currByte, err = reader.ReadByte()
			if err != nil {
				fmt.Println("Error in hashfile2", err)
				break
			}

			// Magic hash
			hash -= uint64(window.getFirstByte()) * math.Pow(HASH_PRIME_ROOT, HASH_WINDOW_SIZE-1)
			hash *= HASH_PRIME_ROOT
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
		arrBlockHash = append(arrBlockHash, BlockHash{Length: lenCurr, Hash: hashBlock, PositionInFile: startWindowPosition})
	}

	fmt.Printf("Found %d matches!\n", cmatch)
	fmt.Printf("Went through %d bytes!\n", c)

	return arrBlockHash
}
