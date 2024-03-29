package main

// TODO: Add modulo to use smaller int and prevent buffer overflow
// TODO: Optimize algo, bit shifting instead of modulo?
// TODO: Have currBlock and currByte share same underlying array for memory optimization?
// TODO: Profile CPU & Memory usage
// TODO: Log it all!
// TODO: Above logs, and errors prints, check to standardize
// TODO: Make it a clean architecture (ask a gopher)
// TODO: Create initialize (all?) struct with New*** (ask a gopher)
// TODO: Check better to declare global variables or pass through all methods (i.e. chanClientChange, chanClientAdd) (ask a gopher)
// TODO: Check TCP connections better to reconnect, keep live, heartbeat? (ask a gopher)
// TODO: New empty folders, also sync. Right now folders are synced when new content updated.
// TODO: Comment them all!
// TODO: Folder renaming, file renaming
// TODO: Folder and file remove
// TODO: Connection to client also use for once in a while ping and remove if disconnect
// TODOING: Defer all file closing?
// TODOING: Handle errors, especially from readFull (when we still have bytes but have reached the end)
// TODONE: When reading, need to remember what is the current length in window (say we read less than len(window))
// TODONE: Variable names, decide case style
// TODO FEATURE: Need file versioning, especially for conflict handling
// TODO FEATURE: Related to above, client needs current file state, to get updated state from a specific version
// TODO FEATURE: Port in config
// TODO FEATURE: Compress data before sending
// TODO FEATURE: Detect moving file with couple hashes as to not transfer again
// TODO FEATURE: Shared Folders
// TODO FEATURE: Possibly ping every so often client for updates and vice versa
// TODO FEATURE: For the above, another addition would be to queue (and group similar fast) updates from client
// Tests: No file on either end (2 tests)
// Tests: No folder on either end (2 tests)
// Tests: Properly terminate/restart connection (on client) when one end closes
// Tests: Properly terminate/restart connection (on server) when client leaves
// Tests: Add/remove char in the beginning, middle, end, random place
// Tests: Add/remove 2 chars in the beginning, middle, end, random place
// Tests: Same files
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
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"

	"thierry/sync/hasher"
)

var configuration Configuration

// TODO: UpdaterClientId create and use to not update same originator client file
// TODO: Update ip and use id instead (multiple users same ip)
type FileHashResult struct {
	StrRelativeFilepath string
	UpdaterClientId     string
	ArrBlockHash        []hasher.BlockHash
	// Indicate if this is a client update request
	IsClientUpdate bool
	// Indicates if this is a delete file/folder request
	IsDelete bool
}

type FileChangeList struct {
	ArrFileChange []hasher.FileChange
}

type ClientConnection struct {
	id      int
	isInUse bool
	conn    net.Conn
	encoder *gob.Encoder
	decoder *gob.Decoder
}

type Configuration struct {
	RootDir string
}

// handleConnection sends/receives data to a client's connection
// The same connection is used to update a client file remotely
// or to receive a modified file from client
// Note: We currently don't handle multiple changes at once once
// a single connection. They should be batched one by one.
func handleConnection(conn net.Conn,
	chanClientChange chan *FileHashResult,
	chanClientAdd chan *ClientConnection) {
	// defer conn.Close()
	fmt.Println("Accepted a connection from ", conn.RemoteAddr())
	encoder := gob.NewEncoder(conn)
	decoder := gob.NewDecoder(conn)
	client := ClientConnection{conn: conn, encoder: encoder, decoder: decoder}

	// Check if we're receiving a client update or ping for update
	fileHashResult := &FileHashResult{}
	err := decoder.Decode(fileHashResult)
	if err != nil {
		fmt.Println("Connection error from client (check receive/ping): ", err, conn.RemoteAddr().String())
		conn.Close()
		return
	}

	// Split here, we're receiving data from client
	if fileHashResult.IsClientUpdate == true {
		if fileHashResult.IsDelete == true {
			processDeleteFromClient(client, fileHashResult, chanClientChange)
		} else {
			processUpdateFromClient(client, fileHashResult, chanClientChange)
		}
	} else {
		// Add new client into our client list
		chanClientAdd <- &client
	}
}

func processUpdateFromClient(client ClientConnection,
	fileHashResult *FileHashResult,
	chanClientChange chan *FileHashResult) {
	strAbsoluteFilepath := configuration.RootDir + filepath.FromSlash(fileHashResult.StrRelativeFilepath)

	// Check if file exists, if not create it
	// TODO: Possible optimization here, skip all processes just upload it
	if hasher.CreateFileIfNotExists(strAbsoluteFilepath) != true {
		fmt.Println("Error while creating local file, aborting update from client", strAbsoluteFilepath)
		return
	}

	// We do our hashing
	fmt.Println("Do hashing of file", strAbsoluteFilepath)
	fmt.Println("Received from client ", *fileHashResult)
	var arrBlockHash []hasher.BlockHash
	arrBlockHash = hasher.HashFile(strAbsoluteFilepath)
	fmt.Println("Server hashing file: ", arrBlockHash)

	// Compare two files
	var arrFileChange []hasher.FileChange
	arrFileChange = hasher.CompareFileHashes(fileHashResult.ArrBlockHash, arrBlockHash)
	// TODO: If no changes on server, just skip the rest from here
	fmt.Printf("We found %d changes!\n", len(arrFileChange))

	// Get difference data from client
	fmt.Println("2. Sending arrFileChange", arrFileChange)
	err := client.encoder.Encode(FileChangeList{ArrFileChange: arrFileChange})
	if err != nil {
		log.Fatal("Connection error from client (get diff data): ", err, client.conn.RemoteAddr())
	}

	// Receive updated differences from client
	err = client.decoder.Decode(&arrFileChange)
	fmt.Println("5. decoded")
	fmt.Println(arrFileChange)
	if err != nil {
		log.Fatal("Connection error from client (received updated diff): ", err, client.conn.RemoteAddr())
	}

	// Update destination file
	hasher.UpdateDestinationFile(arrFileChange, strAbsoluteFilepath, configuration.RootDir)

	// Send update to all other clients
	fileHashResult.UpdaterClientId = strings.Split(client.conn.RemoteAddr().String(), ":")[0]
	chanClientChange <- fileHashResult
}

func processDeleteFromClient(client ClientConnection,
	fileHashResult *FileHashResult,
	chanClientChange chan *FileHashResult) {
	strAbsoluteFilepath := configuration.RootDir + filepath.FromSlash(fileHashResult.StrRelativeFilepath)

	// Check if file exists, if not skip it all
	if _, err := os.Stat(strAbsoluteFilepath); err != nil {
		fmt.Println("File to delete from client doesn't exist, skip it locally", strAbsoluteFilepath)

	} else {
		// Delete file on server
		fmt.Println("Removing all under", strAbsoluteFilepath)
		err := os.RemoveAll(strAbsoluteFilepath)
		if err != nil {
			fmt.Println("There was an error removing...", strAbsoluteFilepath)
		}
	}

	// Send update to all other clients
	fileHashResult.UpdaterClientId = strings.Split(client.conn.RemoteAddr().String(), ":")[0]
	chanClientChange <- fileHashResult
}

func processUpdateToClient(client *ClientConnection,
	fileHashResult *FileHashResult,
	chanClientRemove chan *ClientConnection) {
	// Mark as in use
	// TODO: Check if really needed?
	if client.isInUse {
		fmt.Println("Client currently in use", client.conn.RemoteAddr())
		return
	}
	client.isInUse = true
	fmt.Println("Do update to client ", client.conn.RemoteAddr())
	strAbsoluteFilepath := configuration.RootDir + filepath.ToSlash(fileHashResult.StrRelativeFilepath)

	// Send change from server into client

	// 1. Sending result to client for update
	err := client.encoder.Encode(fileHashResult)
	if err != nil {
		chanClientRemove <- client
		fmt.Println("Client disconnected (processUpdateToClient/sending result)", err, client.conn.RemoteAddr().String())
		return
	}
	fmt.Println("1. Sending to client...", client.conn.RemoteAddr())
	fmt.Println(fileHashResult.ArrBlockHash)

	// 4. Receive list of differences from client
	arrFileChange := &FileChangeList{}
	err = client.decoder.Decode(arrFileChange)
	fmt.Println("4. Received arrFileChange from client", client.conn.RemoteAddr())
	fmt.Println(*arrFileChange)
	if err != nil {
		chanClientRemove <- client
		fmt.Println("Client disconnected (processUpdateToClient/receiving diff)", err, client.conn.RemoteAddr().String())
		return
	}
	hasher.UpdateDeltaData(arrFileChange.ArrFileChange, strAbsoluteFilepath)
	fmt.Println("Updated with delta")
	fmt.Println(arrFileChange)
	// 5. Resending updated data
	err = client.encoder.Encode(arrFileChange)
	fmt.Println("5. Resending to client", client.conn.RemoteAddr())
	fmt.Println(*arrFileChange)
	if err != nil {
		chanClientRemove <- client
		fmt.Println("Client disconnected (processUpdateToClient/resending updated data)", err, client.conn.RemoteAddr().String())
		return
	}

	fmt.Println("Done with client", client.conn.RemoteAddr().String())
	client.isInUse = false
}

func processDeleteToClient(client *ClientConnection,
	fileHashResult *FileHashResult,
	chanClientRemove chan *ClientConnection) {
	// Mark as in use
	// TODO: Check if really needed?
	if client.isInUse {
		fmt.Println("Client currently in use", client.conn.RemoteAddr())
		return
	}
	client.isInUse = true
	fmt.Println("Do delete to client ", client.conn.RemoteAddr())

	// Send delete from server into client

	// 1. Sending result to client for update
	fmt.Println("1. Sending delete to client...", client.conn.RemoteAddr())
	err := client.encoder.Encode(fileHashResult)
	if err != nil {
		chanClientRemove <- client
		fmt.Println("Client disconnected (processUpdateToClient/sending result)", err, client.conn.RemoteAddr().String())
		return
	}
	fmt.Println("Done with client", client.conn.RemoteAddr().String())
	client.isInUse = false
}

// prepareUpdateToClient .......
// also pre-hashes file server side
func prepareUpdateToClient(fileHashResult *FileHashResult) {
	// Hashing file on our end
	fmt.Println("Preparing new hash file for client update")
	strAbsoluteFilepath := configuration.RootDir + filepath.FromSlash(fileHashResult.StrRelativeFilepath)
	fileHashResult.ArrBlockHash = hasher.HashFile(strAbsoluteFilepath)
}

func processAllClients(chanClientChange chan *FileHashResult, chanClientAdd chan *ClientConnection, chanClientRemove chan *ClientConnection) {
	clients := make(map[string]*ClientConnection)

	// TODO: Less nested loops here
	for {
		select {
		// Process client changes
		case fileHashResult := <-chanClientChange:
			prepareUpdateToClient(fileHashResult)
			fmt.Println("New change: ", fileHashResult)
			for _, client := range clients {
				if fileHashResult.UpdaterClientId == strings.Split(client.conn.RemoteAddr().String(), ":")[0] {
					continue
				}
				fmt.Printf("Sending update to client ip %s, original client ip %s\n", client.conn.RemoteAddr(), fileHashResult.UpdaterClientId)
				if fileHashResult.IsDelete == true {
					go processDeleteToClient(client, fileHashResult, chanClientRemove)
				} else {
					go processUpdateToClient(client, fileHashResult, chanClientRemove)
				}
			}
			// Process new clients
		case client := <-chanClientAdd:
			fmt.Println("New client: ", client.conn.RemoteAddr())
			clients[strings.Split(client.conn.RemoteAddr().String(), ":")[0]] = client
			// When clients logs off
		case client := <-chanClientRemove:
			fmt.Printf("Client disconnects: %v\n", client.conn.RemoteAddr().String())
			delete(clients, strings.Split(client.conn.RemoteAddr().String(), ":")[0])
		}
	}
}

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
var memprofile = flag.String("memprofile", "", "write memory profile to this file")

func main() {

	file, err := os.Open("conf.json")
	if err != nil {
		log.Fatal("Could not load config file")
	}
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&configuration)
	if err != nil {
		fmt.Println("error loading config:", err)
	}
	fmt.Println("Root dir setup:", configuration.RootDir)

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

	// Changes from originator updater client will be funneled here
	chanClientChange := make(chan *FileHashResult)
	// Clients to add will be funneled here
	chanClientAdd := make(chan *ClientConnection)
	// Clients to remove will be funneled here
	chanClientRemove := make(chan *ClientConnection)
	go processAllClients(chanClientChange, chanClientAdd, chanClientRemove)

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
}
