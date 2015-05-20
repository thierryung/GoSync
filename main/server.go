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
// TODO: Detect moving file with couple hashes as to not transfer again
// TODO: Create initialize struct with New***
// Feature: Shared Folders
// Tests: No file on either end (2 tests)
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
	FileHashParam  hasher.FileHashParam
	ArrBlockHash   []hasher.BlockHash
	IsClientUpdate bool
}

type FileChangeList struct {
	ArrFileChange []hasher.FileChange
}

// handleConnection sends/receives data to a client's connection
// The same connection is used to update a client file remotely
// or to receive a modified file from client
// Note: We currently don't handle multiple changes at once once
// a single connection. They should be batched one by one.
func handleConnection(conn net.Conn) {
	// defer conn.Close()
	fmt.Println("Accepted a connection")
	encoder := gob.NewEncoder(conn)
	decoder := gob.NewDecoder(conn)

	// Check if we're receiving a client update or ping for update
	fileHashResult := &FileHashResult{}
	err := decoder.Decode(fileHashResult)
	if err != nil {
		log.Fatal("Connection error from client (check receive/ping): ", err)
	}
	fmt.Println(*fileHashResult)

	// Split here, we're receiving data from client
	if fileHashResult.IsClientUpdate == true {
		processUpdateFromClient(conn, fileHashResult, encoder, decoder)
	} else {
		// Ping from client
		processPingFromClient(conn, encoder, decoder)
	}
}

func processUpdateFromClient(conn net.Conn, fileHashResult *FileHashResult, encoder *gob.Encoder, decoder *gob.Decoder) {
	// We do our hashing
	fmt.Println("Do hashing")
	var arrBlockHash []hasher.BlockHash
	arrBlockHash = hasher.HashFile(fileHashResult.FileHashParam)

	// Compare two files
	var arrFileChange []hasher.FileChange
	arrFileChange = hasher.CompareFileHashes(fileHashResult.ArrBlockHash, arrBlockHash)
	fmt.Printf("We found %d changes!\n", len(arrFileChange))
	fmt.Println(arrFileChange)

	// Get difference data from client
	err := encoder.Encode(FileChangeList{ArrFileChange: arrFileChange})
	if err != nil {
		log.Fatal("Connection error from client (get diff data): ", err)
	}

	// Receive updated differences from client
	err = decoder.Decode(&arrFileChange)
	if err != nil {
		log.Fatal("Connection error from client (received updated diff): ", err)
	}
	fmt.Println("decoded")
	fmt.Println(arrFileChange)

	// Update destination file
	hasher.UpdateDestinationFile(arrFileChange, fileHashResult.FileHashParam)
}

func processPingFromClient(conn net.Conn, encoder *gob.Encoder, decoder *gob.Decoder) {
	// We do our pinging
	fmt.Println("Do pinging")

	// Check what files have changed
	// Send change from server into client

	// Simulation: Hashing file on our end
	var strFilepath string = "/home/thierry/projects/volfromserver.txt"
	var arrBlockHash []hasher.BlockHash
	var fileHashParam hasher.FileHashParam
	fileHashParam = hasher.FileHashParam{Filepath: strFilepath}
	arrBlockHash = hasher.HashFile(fileHashParam)
	// Sending result to server for update
	err := encoder.Encode(FileHashResult{FileHashParam: hasher.FileHashParam{Filepath: strFilepath}, ArrBlockHash: arrBlockHash})
	if err != nil {
		log.Fatal("Connection error from server (processPingFromClient/sending result): ", err)
	}
	fmt.Println("Sending to client...")
	fmt.Println(arrBlockHash)

	// Receive list of differences from server
	arrFileChange := &FileChangeList{}
	err = decoder.Decode(arrFileChange)
	if err != nil {
		log.Fatal("Connection error from server (processPingFromClient/receiving diff): ", err)
	}
	fmt.Println("received from client")
	fmt.Println(*arrFileChange)
	hasher.UpdateDeltaData(arrFileChange.ArrFileChange, fileHashParam)
	// Resending updated data
	err = encoder.Encode(arrFileChange.ArrFileChange)
	if err != nil {
		log.Fatal("Connection error from server (processPingFromClient/resending updated data): ", err)
	}
	fmt.Println("Resent to client")
	fmt.Println(*arrFileChange)
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
