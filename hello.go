package main

// TODO: Add modulo to use smaller int and prevent buffer overflow
// TODO: Optimize algo, bit shifting instead of modulo?
// TODO: Handle errors
// TODO: When reading, need to remember what is the current length in window (say we read less than len(window))
// TODO: Defer file closing
// TODO: Have currBlock and currByte share same underlying array for memory optimization
// TODO: Profile CPU & Memory usage

import (
	"bufio"
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
	// The current bytes for this block (meaning at least the size of currBytes)
	currBlock  []byte
	startIndex int
	length     int
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
	var c, index, cmatch, lenmin, lenmax, lencurr int = 0, 0, 0, -1, -1, -1
	var under, _100, _200, _300, _400, _500, _plus int = 0, 0, 0, 0, 0, 0, 0
	var hash, primeRoot uint64 = 0, 31
	var windowSize = 1024
	var mask uint64 = (1 << 19) - 1
	var currByte byte
	var window WindowBytes
	window.init(windowSize)

	// Read file
	f, err := os.Open("/home/thierry/projects/vol")
	check(err)
	reader := bufio.NewReader(f)

	// Reset the read window, we'll slide from there
	lencurr, err = window.readFull(reader)
	c += lencurr
	// Calculate window hash (first time
	for index, currByte = range window.currBytes {
		hash += uint64(currByte) * iPow(primeRoot, windowSize-index-1)
	}

	for {
		// Check if we fit the match, and at least a certain amount of bytes
		if (hash | mask) == hash {
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

			cmatch++

			// Reset the read window, we'll slide from there
			lencurr, err = window.readFull(reader)
			if err != nil {
				break
			}
			c += lencurr
			// Calculate window hash
			for index, currByte = range window.currBytes {
				hash += uint64(currByte) * iPow(primeRoot, windowSize-index-1)
			}

		} else {
			// No fit, we keep going for this block
			currByte, err = reader.ReadByte()
			if err != nil {
				break
			}

			// Magic hash
			hash -= uint64(window.getFirstByte()) * iPow(primeRoot, windowSize-1)
			hash *= primeRoot
			hash += uint64(currByte)

			if (c % 10000000) == 0 {
				fmt.Printf("currBlock length %d, cap %d\n", len(window.currBlock), cap(window.currBlock))
			}

			if hash <= 0 {
				fmt.Println("*** BAD")
				fmt.Println(window.getFirstByte())
				fmt.Println(uint64(window.getFirstByte()) * iPow(primeRoot, windowSize-1))
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

	if *memprofile != "" {
		f1, err := os.Create(*memprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.WriteHeapProfile(f1)
		f1.Close()
	}
	// TODO: Get last block
	f.Close()
	fmt.Printf("Found %d matches!\n", cmatch)
	fmt.Printf("Went through %d bytes!\n", c)
	fmt.Printf("Min block %d bytes!\n", lenmin)
	fmt.Printf("Max block %d bytes!\n", lenmax)
	fmt.Printf("%d, %d, %d, %d, %d, %d, %d\n", under, _100, _200, _300, _400, _500, _plus)
	elapsed := time.Since(start)
	fmt.Printf("Binomial took %s\n", elapsed)
}

