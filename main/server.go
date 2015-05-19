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
// TODO: Port in config
// TODO: Somehow make use of channels and (better?) concurrent design
// TODO: Log it all!
// TODO: Above logs, and errors prints, check to standardize
// TODO: Compress data before sending?
// Feature: Shared Folders
// Tests: Add/remove char in the beginning, middle, end, random place
// Tests: Add/remove 2 chars in the beginning, middle, end, random place
// Tests: Add/remove multiple chars in random places
// Tests: Unit tests!

import (
	//"encoding/gob"
	"flag"
	"fmt"
	"log"
	//"net"
	"crypto/rand"
	"crypto/tls"
	"encoding/gob"
	"net"
	"os"
	"runtime/pprof"
	"time"

	"thierry/sync/hasher"
)

type FileHashResult struct {
	FileHashParam hasher.FileHashParam
	ArrBlockHash  []hasher.BlockHash
}

type FileChangeList struct {
	ArrFileChange []hasher.FileChange
}

func handleConnection(conn net.Conn) {
	// defer conn.Close()
	fmt.Println("Accepted a connection")
	decoder := gob.NewDecoder(conn)

	// Get the path of changed file
	fileHashResult := &FileHashResult{}
	err := decoder.Decode(fileHashResult)
	if err != nil {
		log.Fatal("Connection error from client: ", err)
	}
	fmt.Println(*fileHashResult)

	fmt.Println("Do hashing")
	// We do our hashing
	var arrBlockHash []hasher.BlockHash
	arrBlockHash = hasher.HashFile(fileHashResult.FileHashParam)

	// Compare two files
	var arrFileChange []hasher.FileChange
	arrFileChange = hasher.CompareFileHashes(fileHashResult.ArrBlockHash, arrBlockHash)
	fmt.Printf("We found %d changes!\n", len(arrFileChange))
	fmt.Println(arrFileChange)

	// Get difference data from client
	encoder := gob.NewEncoder(conn)
	err = encoder.Encode(FileChangeList{ArrFileChange: arrFileChange})
	if err != nil {
		log.Fatal("Connection error from client: ", err)
	}

	// Receive updated differences from client
	err = decoder.Decode(&arrFileChange)
	if err != nil {
		log.Fatal("Connection error from client: ", err)
	}
	fmt.Println("decoded")
	fmt.Println(arrFileChange)

	// Update destination file
	hasher.UpdateDestinationFile(arrFileChange, fileHashResult.FileHashParam)

	// Send from server into client
	// encoder := gob.NewEncoder(conn)
	// p2 := P{"testttt from server"}
	// fmt.Println(p2)
	// encoder.Encode(p2)
}

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
var memprofile = flag.String("memprofile", "", "write memory profile to this file")

func main() {
	/*
	     // var strFilepath string = "/home/thierry/projects/vol1"
	   	// var strFilepath2 string = "/home/thierry/projects/vol2"
	   	var strFilepath1 string = "/home/thierry/projects/volclientlocal.txt"
	   	var strFilepath2 string = "/home/thierry/projects/volserverlocal.txt"

	   	var arrBlockHash []hasher.BlockHash
	   	var fileHashParam hasher.FileHashParam
	   	var arrBlockHash2 []hasher.BlockHash
	   	var fileHashParam2 hasher.FileHashParam
	   	var arrFileChange []hasher.FileChange

	   	// Hash file 1
	   	fileHashParam = hasher.FileHashParam{Filepath: strFilepath1}
	   	arrBlockHash = hasher.HashFile(fileHashParam)

	   	// Hash file 2
	   	fileHashParam2 = hasher.FileHashParam{Filepath: strFilepath2}
	   	arrBlockHash2 = hasher.HashFile(fileHashParam2)

	   	// Compare two files
	   	arrFileChange = hasher.CompareFileHashes(arrBlockHash, arrBlockHash2)
	   	fmt.Printf("We found %d changes!\n", len(arrFileChange))
	   	fmt.Println(arrFileChange)
	   	fmt.Println(arrFileChange[0].LengthToAdd)

	   	// Get difference data
	   	hasher.UpdateDeltaData(arrFileChange, fileHashParam)
	   	fmt.Println(arrFileChange)

	   	// Update destination file
	   	hasher.UpdateDestinationFile(arrFileChange, fileHashParam2)
	     os.Exit(0) */

	fmt.Println("Starting server...")

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

	// Start the server with tls certificates
	cert, err := tls.LoadX509KeyPair("../cert/cert2pem.pem", "../cert/key2.pem")
	if err != nil {
		fmt.Printf("Error server loadkeys: %s", err)
		return
	}
	config := tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS10}

	config.Rand = rand.Reader
	ln, err := tls.Listen("tcp", ":8080", &config)
	if err != nil {
		fmt.Printf("Error server listen: %s", err)
		return
	}
	fmt.Println("Accepting connections...")
	for {
		conn, err := ln.Accept() // this blocks until connection or error
		if err != nil {
			fmt.Printf("Error server accept: %s", err)
			continue
		}
		defer conn.Close()
		go handleConnection(conn) // a goroutine handles conn so that the loop can accept other connections
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
