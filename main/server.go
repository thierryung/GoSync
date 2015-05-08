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
	"flag"
	"fmt"
	"log"
	"os"
	"runtime/pprof"
	"time"
	//"encoding/gob"
	//"net"

	"thierry/sync/hasher"
)

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
var memprofile = flag.String("memprofile", "", "write memory profile to this file")

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

	var arrBlockHash []hasher.BlockHash
	var fileHashParam hasher.FileHashParam
	var arrBlockHash2 []hasher.BlockHash
	var fileHashParam2 hasher.FileHashParam
	var arrFileChange []hasher.FileChange

	// Hash file 1
	fileHashParam = hasher.FileHashParam{Filepath: strFilepath1, WindowSize: windowSize, PrimeRoot: primeRoot, Mask: mask}
	arrBlockHash = hasher.HashFile(fileHashParam)

	// Hash file 2
	fileHashParam2 = hasher.FileHashParam{Filepath: strFilepath2, WindowSize: windowSize, PrimeRoot: primeRoot, Mask: mask}
	arrBlockHash2 = hasher.HashFile(fileHashParam2)

	// Compare two files
	arrFileChange = hasher.CompareFileHashes(arrBlockHash, arrBlockHash2)
	//fmt.Println(arrFileChange)
	fmt.Printf("We found %d changes!\n", len(arrFileChange))

	// Get difference data
	hasher.UpdateDeltaData(arrFileChange, fileHashParam)

	// Update destination file
	hasher.UpdateDestinationFile(arrFileChange, fileHashParam2)

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
