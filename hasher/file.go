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
	length         int
	hash           [16]byte
	positionInFile int
}

func CalculateLengthBetween(b []BlockHash, start, end int) int {
	var result int = 0
	for i := start; i < end; i++ {
		result += b[i].length
	}
	return result
}

func HashFile(param FileHashParam) []BlockHash {
	var c, startWindowPosition, index, cmatch, lenMin, lenMax, lenCurr int = 0, 0, 0, 0, -1, -1, -1
	var under, _100, _200, _300, _400, _500, _plus int = 0, 0, 0, 0, 0, 0, 0
	var hash uint64 = 0
	var currByte byte
	var window WindowBytes
	var hashBlock [16]byte
	var arrBlockHash []BlockHash

	window.init(param.WindowSize)

	// Read file
	f, err := os.Open(param.Filepath)
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
		hash += uint64(currByte) * math.Pow(param.PrimeRoot, param.WindowSize-index-1)
	}

	for {
		// Check if we fit the match, and at least a certain amount of bytes
		if (hash | param.Mask) == hash {
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
				hash += uint64(currByte) * math.Pow(param.PrimeRoot, param.WindowSize-index-1)
			}

		} else {
			// No fit, we keep going for this block
			currByte, err = reader.ReadByte()
			if err != nil {
				break
			}

			// Magic hash
			hash -= uint64(window.getFirstByte()) * math.Pow(param.PrimeRoot, param.WindowSize-1)
			hash *= param.PrimeRoot
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
