package main

// TODO: Add modulo to use smaller int and prevent buffer overflow
// TODO: Optimize algo, bit shifting instead of modulo?
// TODO: Handle errors, especially from readFull (when we still have bytes but have reached the end)
// TODO: When reading, need to remember what is the current length in window (say we read less than len(window))
// TODO: Defer file closing
// TODO: Have currBlock and currByte share same underlying array for memory optimization
// TODO: Profile CPU & Memory usage

import (
	"bufio"
	"crypto/md5"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime/pprof"
	"time"
)

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
var memprofile = flag.String("memprofile", "", "write memory profile to this file")

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

type BlockHash struct {
	length int
	hash   [16]byte
}

type FileHashParam struct {
	filepath           string
	windowSize            int
	hash, primeRoot, mask uint64
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

func check(e error) {
	if e != nil {
		panic(e)
	}
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

func hashFile(param FileHashParam) []BlockHash {
	var c, index, cmatch, lenmin, lenmax, lencurr int = 0, 0, 0, -1, -1, -1
	var under, _100, _200, _300, _400, _500, _plus int = 0, 0, 0, 0, 0, 0, 0
	var hash uint64 = 0
	var currByte byte
	var window WindowBytes
	var hashblock [16]byte
	var arrBlockHash []BlockHash

	window.init(param.windowSize)

	// Read file
	f, err := os.Open(param.filepath)
	check(err)
	reader := bufio.NewReader(f)

	// Reset the read window, we'll slide from there
	lencurr, err = window.readFull(reader)
	c += lencurr
	// Calculate window hash (first time)
	for index, currByte = range window.currBytes {
		hash += uint64(currByte) * iPow(param.primeRoot, param.windowSize-index-1)
	}

	for {
		// Check if we fit the match, and at least a certain amount of bytes
		if (hash | param.mask) == hash {
			if lenmax == -1 || lencurr > lenmax {
				lenmax = lencurr
			}
			if lenmin == -1 || lencurr < lenmin {
				lenmin = lencurr
			}

			if lencurr < 50000 {
				under++
			} else if lencurr < 100000 {
				_100++
			} else if lencurr < 200000 {
				_200++
			} else if lencurr < 300000 {
				_300++
			} else if lencurr < 400000 {
				_400++
			} else if lencurr < 600000 {
				_500++
			} else {
				_plus++
			}

			// New match, md5 it
			cmatch++
			hashblock = md5.Sum(window.currBlock)
			arrBlockHash = append(arrBlockHash, BlockHash{length: lencurr, hash: hashblock})
			//fmt.Printf("%x\n", hashblock)
			//fmt.Printf("%s\n\n", window.currBlock)

			// Reset the read window, we'll slide from there
			lencurr, err = window.readFull(reader)
			c += lencurr
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

			if (c % 10000000) == 0 {
				//fmt.Printf("currBlock length %d, cap %d\n", len(window.currBlock), cap(window.currBlock))
			}

			if hash <= 0 {
				fmt.Println("*** BAD")
				fmt.Println(window.getFirstByte())
				fmt.Println(uint64(window.getFirstByte()) * iPow(param.primeRoot, param.windowSize-1))
				fmt.Println(hash)
				fmt.Println(c)
				os.Exit(0)
			}

			// Add new byte read
			window.addByte(currByte)
			c++
			lencurr++
		}
	}

	// Last block
	hashblock = md5.Sum(window.currBlock)
	arrBlockHash = append(arrBlockHash, BlockHash{length: lencurr, hash: hashblock})
	fmt.Printf("%x\n", hashblock)
	fmt.Printf("%s\n\n", window.currBlock)

	// TODO: Get last block
	f.Close()
	fmt.Printf("Found %d matches!\n", cmatch)
	fmt.Printf("Went through %d bytes!\n", c)
	fmt.Printf("Min block %d bytes!\n", lenmin)
	fmt.Printf("Max block %d bytes!\n", lenmax)
	fmt.Printf("%d, %d, %d, %d, %d, %d, %d\n", under, _100, _200, _300, _400, _500, _plus)

	return arrBlockHash
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
	//var windowSize = 1024
	//var mask uint64 = (1 << 19) - 1
	var windowSize = 3
	var mask uint64 = (1 << 2) - 1

	// Init with 1000
	// TODO: Have all under variables for better configuration
	var arrBlockHash []BlockHash
	var fileHashParam FileHashParam
	var arrBlockHash2 []BlockHash
	var fileHashParam2 FileHashParam

	fileHashParam = FileHashParam{filepath: "/home/thierry/projects/vol.test", windowSize: windowSize, primeRoot: primeRoot, mask: mask}
	arrBlockHash = hashFile(fileHashParam)

	for key, val := range arrBlockHash {
    fmt.Printf("%d, %x\n", key, val)
  }

	fileHashParam2 = FileHashParam{filepath: "/home/thierry/projects/vol2.test", windowSize: windowSize, primeRoot: primeRoot, mask: mask}
	arrBlockHash2 = hashFile(fileHashParam2)

	for key, val := range arrBlockHash2 {
    fmt.Printf("%d, %x\n", key, val)
  }

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
