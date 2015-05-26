package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/fsnotify.v1"
	"thierry/sync/hasher"
	"thierry/sync/watch"
)

var done chan bool = make(chan bool)

type FileHashResult struct {
	FileHashParam  hasher.FileHashParam
	ArrBlockHash   []hasher.BlockHash
	IsClientUpdate bool
}

type FileChangeList struct {
	ArrFileChange []hasher.FileChange
}

// monitorLocalChanges will wait for io changes
// and send data to server
func monitorLocalChanges(cafile string, server string) {
	fmt.Println("*** Recursively monitoring folder")
	watcher, err := watch.NewRecursiveWatcher("/home/thierry/projects/testdata/")
	if err != nil {
		log.Println("Watcher create error : ", err)
	}
	defer watcher.Close()
	_done := make(chan bool)

	go func() {
		for {
			select {
			case event := <-watcher.Events:
				switch {
				// create a file/directory
				case event.Op&fsnotify.Create == fsnotify.Create:
					fi, err := os.Stat(event.Name)
					if err != nil {
						// eg. stat .subl513.tmp : no such file or directory
						fmt.Println(err)
					} else if fi.IsDir() {
						fmt.Println("Detected new directory %s", event.Name)
						if !watch.ShouldIgnoreFile(filepath.Base(event.Name)) {
							watcher.AddFolder(event.Name)
						}
					} else {
						fmt.Println("Detected new file %s", event.Name)
						// watcher.Files <- event.Name // created a file
						// Commented out since we don't read from new file
					}

				case event.Op&fsnotify.Write == fsnotify.Write:
					// modified a file, assuming that you don't modify folders
					fmt.Println("Detected file modification %s", event.Name)
					// TODO: Remove, but for now don't handle .tmp files
					if strings.Index(event.Name, ".tmp") == -1 {
						watcher.Files <- event.Name
						log.Println("Modified file: ", event.Name)
						connsender := connectToServer(cafile, server)
						go sendClientChanges(connsender, event.Name)
					}
				case event.Op&fsnotify.Remove == fsnotify.Remove:
					log.Println("Removed file: ", event.Name)
				case event.Op&fsnotify.Rename == fsnotify.Rename:
					log.Println("Renamed file: ", event.Name)
				case event.Op&fsnotify.Chmod == fsnotify.Chmod:
					log.Println("File changed permission: ", event.Name)
				}

			case err := <-watcher.Errors:
				log.Println("Watcher watching error : ", err)
				_done <- true
				done <- true
			}
		}

	}()

	<-_done
}

func sendClientChanges(conn net.Conn, strFilepath string) {
	defer conn.Close()
	// Hashing file on our end
	var arrBlockHash []hasher.BlockHash
	var fileHashParam hasher.FileHashParam
	fileHashParam = hasher.FileHashParam{Filepath: strFilepath}
	arrBlockHash = hasher.HashFile(fileHashParam)
	// Sending result to server for update
	encoder := gob.NewEncoder(conn)
	err := encoder.Encode(FileHashResult{FileHashParam: hasher.FileHashParam{Filepath: strFilepath}, ArrBlockHash: arrBlockHash, IsClientUpdate: true})
	if err != nil {
		log.Fatal("Connection error from client (sendclientchanges/sending result): ", err)
	}
	fmt.Println("Sending to server...")
	fmt.Println(arrBlockHash)

	// Receive list of differences from server
	arrFileChange := &FileChangeList{}
	decoder := gob.NewDecoder(conn)
	err = decoder.Decode(arrFileChange)
	if err != nil {
		log.Fatal("Connection error from client (sendclientchanges/receiving diff): ", err)
	}
	fmt.Println("received from server")
	fmt.Println(*arrFileChange)
	hasher.UpdateDeltaData(arrFileChange.ArrFileChange, fileHashParam)
	// Resending updated data
	err = encoder.Encode(arrFileChange.ArrFileChange)
	if err != nil {
		log.Fatal("Connection error from client (sendclientchanges/resending updated data): ", err)
	}
	fmt.Println("Resent to server")
	fmt.Println(*arrFileChange)
}

func receiveServerChanges(conn net.Conn) {
	// Sending result to server for update
	encoder := gob.NewEncoder(conn)
	err := encoder.Encode(FileHashResult{IsClientUpdate: false})
	if err != nil {
		log.Fatal("Connection error from client (receive server change): ", err)
	}

	decoder := gob.NewDecoder(conn)
	fileHashResult := &FileHashResult{}

	for {
		// Get file update
		err = decoder.Decode(fileHashResult)
		if err != nil {
			log.Fatal("Connection error from client (get file update): ", err)
		}
		fmt.Println("Receiving from server")
		fmt.Println(*fileHashResult)

		// We do our hashing
		fmt.Println("Do hashing from client")
		var arrBlockHash []hasher.BlockHash
		arrBlockHash = hasher.HashFile(fileHashResult.FileHashParam)
		fmt.Println(arrBlockHash)

		// Compare two files
		var arrFileChange []hasher.FileChange
		arrFileChange = hasher.CompareFileHashes(fileHashResult.ArrBlockHash, arrBlockHash)
		fmt.Printf("We found %d changes!\n", len(arrFileChange))
		fmt.Println(arrFileChange)

		// Send difference data from client
		err = encoder.Encode(FileChangeList{ArrFileChange: arrFileChange})
		if err != nil {
			log.Fatal("Connection error from client (get diff data): ", err)
		}

		// Receive updated differences from server
		arrFileChangeList := &FileChangeList{}
		err = decoder.Decode(arrFileChangeList)
		if err != nil {
			log.Fatal("Connection error from client (received updated diff): ", err)
		}
		fmt.Println("decoded")
		fmt.Println(arrFileChangeList)

		// Update destination file
		hasher.UpdateDestinationFile(arrFileChangeList.ArrFileChange, fileHashResult.FileHashParam)
	}
}

func connectToServer(cafile string, server string) net.Conn {
	// Init connection with global CA
	certs := x509.NewCertPool()
	pemData, err := ioutil.ReadFile(cafile)
	if err != nil {
		log.Fatal("Connection error from client: ", err)
		return nil
	}
	certs.AppendCertsFromPEM(pemData)
	config := tls.Config{RootCAs: certs}

	conn, err := tls.Dial("tcp", server, &config)
	if err != nil {
		log.Fatal("Connection error from client: ", err)
		return nil
	}
	fmt.Println("Connected to server")
	return conn
}

func main() {
	fmt.Println("Starting client...")
	connreceiver := connectToServer("capem.pem", "192.168.216.128:8080")
	go receiveServerChanges(connreceiver)
	monitorLocalChanges("../cert/capem.pem", "192.168.216.128:8080")

	// For now, sleep 1 second
	// What we really want to do is to block (channel?)
	// until we have a file change locally to send to server
	time.Sleep(1000 * time.Millisecond)

	//conn.Close()
	<-done
	fmt.Println("done")
}
