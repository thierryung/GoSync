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
// TODO: Create initialize (all?) struct with New***
// TODO: Check better to declare global variables or pass through all methods (i.e. chanClientChange, chanClientAdd)
// TODO: Check TCP connections better to reconnect, keep live, heartbeat?
// TODO: No file on either end (2 tests)
// TODO: No folder on either end (2 tests)
// TODO: Properly terminate/restart connection (on client) when one end closes
// TODO: Properly terminate/restart connection (on server) when client leaves
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

// TODO: UpdaterClientId create and use to not update same originator client file
// TODO: Update ip and use id instead (multiple users same ip)
type FileHashResult struct {
	FileHashParam   hasher.FileHashParam
	ArrBlockHash    []hasher.BlockHash
	UpdaterClientId string
	IsClientUpdate  bool
}

type FileChangeList struct {
	ArrFileChange []hasher.FileChange
}

type ClientConnection struct {
	id      int
	conn    net.Conn
	encoder *gob.Encoder
	decoder *gob.Decoder
}

// handleConnection sends/receives data to a client's connection
// The same connection is used to update a client file remotely
// or to receive a modified file from client
// Note: We currently don't handle multiple changes at once once
// a single connection. They should be batched one by one.
func handleConnection(conn net.Conn, chanClientChange chan *FileHashResult, chanClientAdd chan *ClientConnection) {
	// defer conn.Close()
	fmt.Println("Accepted a connection from ", conn.RemoteAddr())
	encoder := gob.NewEncoder(conn)
	decoder := gob.NewDecoder(conn)
	client := ClientConnection{conn: conn, encoder: encoder, decoder: decoder}

	// Check if we're receiving a client update or ping for update
	fileHashResult := &FileHashResult{}
	err := decoder.Decode(fileHashResult)
	if err != nil {
		log.Fatal("Connection error from client (check receive/ping): ", err)
	}

	// Split here, we're receiving data from client
	if fileHashResult.IsClientUpdate == true {
		processUpdateFromClient(client, fileHashResult, chanClientChange)
	} else {
		// Add new client into our client list
		chanClientAdd <- &client
	}
}

func processUpdateFromClient(client ClientConnection, fileHashResult *FileHashResult, chanClientChange chan *FileHashResult) {
	// We do our hashing
	fmt.Println("Do hashing")
	fmt.Println("Received from client ", *fileHashResult)
	var arrBlockHash []hasher.BlockHash
	arrBlockHash = hasher.HashFile(fileHashResult.FileHashParam)
	fmt.Println("Server hashing file: ", arrBlockHash)

	// Compare two files
	var arrFileChange []hasher.FileChange
	arrFileChange = hasher.CompareFileHashes(fileHashResult.ArrBlockHash, arrBlockHash)
	fmt.Printf("We found %d changes!\n", len(arrFileChange))
	fmt.Println(arrFileChange)

	// Get difference data from client
	err := client.encoder.Encode(FileChangeList{ArrFileChange: arrFileChange})
	if err != nil {
		log.Fatal("Connection error from client (get diff data): ", err)
	}

	// Receive updated differences from client
	err = client.decoder.Decode(&arrFileChange)
	if err != nil {
		log.Fatal("Connection error from client (received updated diff): ", err)
	}
	fmt.Println("decoded")
	fmt.Println(arrFileChange)

	// Update destination file
	hasher.UpdateDestinationFile(arrFileChange, fileHashResult.FileHashParam)

	// Send update to all other clients
	chanClientChange <- fileHashResult
}

func processUpdateToClient(client *ClientConnection, fileHashResult *FileHashResult) {
	fmt.Println("Do update to client ", client.conn.RemoteAddr())

	// Check what files have changed
	// Send change from server into client

	// Sending result to client for update
	err := client.encoder.Encode(fileHashResult)
	if err != nil {
		log.Fatal("Connection error from server (processUpdateToClient/sending result): ", err)
	}
	fmt.Println("Sending to client...")
	fmt.Println(fileHashResult.ArrBlockHash)

	// Receive list of differences from client
	arrFileChange := &FileChangeList{}
	err = client.decoder.Decode(arrFileChange)
	if err != nil {
		log.Fatal("Connection error from server (processUpdateToClient/receiving diff): ", err)
	}
	fmt.Println("received from client")
	fmt.Println(*arrFileChange)
	hasher.UpdateDeltaData(arrFileChange.ArrFileChange, fileHashResult.FileHashParam)
	// Resending updated data
	err = client.encoder.Encode(arrFileChange.ArrFileChange)
	if err != nil {
		log.Fatal("Connection error from server (processUpdateToClient/resending updated data): ", err)
	}
	fmt.Println("Resent to client")
	fmt.Println(*arrFileChange)
}

// prepareUpdateToClient .......
// also pre-hashes file server side
func prepareUpdateToClient(fileHashResult *FileHashResult) FileHashResult {
	// Hashing file on our end
	fileHashParam := hasher.FileHashParam{Filepath: fileHashResult.FileHashParam.Filepath}
	arrBlockHash := hasher.HashFile(fileHashParam)

	return FileHashResult{FileHashParam: fileHashParam, ArrBlockHash: arrBlockHash}
}

func processAllClients(chanClientChange chan *FileHashResult, chanClientAdd chan *ClientConnection) {
	clients := make(map[string]*ClientConnection)

	for {
		select {
		// Process client changes
		case change := <-chanClientChange:
			fmt.Println("New change: ", change)
			fileHashResult := prepareUpdateToClient(change)
			for ip, client := range clients {
				if ip != client.conn.RemoteAddr().String() {
					fmt.Println("Sending update to client ", client.conn.RemoteAddr())
					go processUpdateToClient(client, &fileHashResult)
				}
			}
			// Process new clients
		case client := <-chanClientAdd:
			fmt.Println("New client: ", client.conn.RemoteAddr())
			clients[client.conn.RemoteAddr().String()] = client
			// case conn := <-rmchan:
			// fmt.Printf("Client disconnects: %v\n", conn)
			// delete(clients, conn)
		}
	}
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
	// Changes from originator updater client will be funneled here
	chanClientChange := make(chan *FileHashResult)
	// Clients to add will be funneled here
	chanClientAdd := make(chan *ClientConnection)
	go processAllClients(chanClientChange, chanClientAdd)

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
		go handleConnection(conn, chanClientChange, chanClientAdd) // a goroutine handles conn so that the loop can accept other connections
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
