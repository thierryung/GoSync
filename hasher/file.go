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
func HashFile(param FileHashParam) []BlockHash {
	var c, startWindowPosition, index, cmatch, lenMin, lenMax, lenCurr int = 0, 0, 0, 0, -1, -1, -1
	var under, _100, _200, _300, _400, _500, _plus int = 0, 0, 0, 0, 0, 0, 0
	var hash uint64 = 0
	var currByte byte
	var window WindowBytes
	var hashBlock [16]byte
	var arrBlockHash []BlockHash

	window.init(HASH_WINDOW_SIZE)

	// Read file
	f, err := os.Open(param.Filepath)
	if err != nil {
		fmt.Println("Err in opening file")
		fmt.Println(err)
		return arrBlockHash
	}
	defer func() {
		if err := f.Close(); err != nil {
			fmt.Println("Err in closing file")
			fmt.Println(err)
			return
		}
	}()
	reader := bufio.NewReader(f)

	// Reset the read window, we'll slide from there
	lenCurr, err = window.readFull(reader)
	if err != nil && lenCurr <= 0 {
		fmt.Println(err)
		fmt.Println("Err in reading file")
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
			arrBlockHash = append(arrBlockHash, BlockHash{Length: lenCurr, Hash: hashBlock, PositionInFile: startWindowPosition})

			// Reset the read window, we'll slide from there
			lenCurr, err = window.readFull(reader)
			// TODO: Check error here? Since readFull can return error
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
	fmt.Printf("Min block %d bytes!\n", lenMin)
	fmt.Printf("Max block %d bytes!\n", lenMax)
	fmt.Printf("%d, %d, %d, %d, %d, %d, %d\n\n", under, _100, _200, _300, _400, _500, _plus)

	return arrBlockHash
}
