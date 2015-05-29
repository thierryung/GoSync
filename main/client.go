package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/fsnotify.v1"
	"thierry/sync/hasher"
	"thierry/sync/watch"
)

var done chan bool = make(chan bool)
var configuration Configuration

type FileHashResult struct {
	StrRelativeFilepath string
	ArrBlockHash        []hasher.BlockHash
	UpdaterClientId     string
	IsClientUpdate      bool
}

type FileChangeList struct {
	ArrFileChange []hasher.FileChange
}

type Configuration struct {
	RootDir      string
	CertFilepath string
	ServerIp     string
}

// monitorLocalChanges will wait for io changes......
func monitorLocalChanges(rootdir string, cafile string, server string) {
	fmt.Println("*** Recursively monitoring folder", rootdir)
	watcher, err := watch.NewRecursiveWatcher(rootdir)
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
						fmt.Println("Detected new directory", event.Name)
						if !watch.ShouldIgnoreFile(filepath.Base(event.Name)) {
							fmt.Println("Monitoring new folder...")
							watcher.AddFolder(event.Name)
							//watcher.Folders <- event.Name
						}
					} else {
						fmt.Println("Detected new file %s", event.Name)
						// watcher.Files <- event.Name // created a file
						// Commented out since we don't read from new file
					}

				case event.Op&fsnotify.Write == fsnotify.Write:
					// modified a file, assuming that you don't modify folders
					fmt.Println("Detected file modification %s", event.Name)
					// TODO: Remove, but for now don't handle .tmp files, nor folders
					fi, err := os.Stat(event.Name)
					if err != nil {
						fmt.Println(err)
					}
					if strings.Index(event.Name, ".tmp") == -1 &&
						strings.Index(event.Name, ".orig") == -1 &&
						strings.Index(event.Name, ".swp") == -1 &&
						fi.Mode().IsRegular() {
						// watcher.Files <- event.Name
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

func sendClientChanges(conn net.Conn, strAbsoluteFilepath string) {
	// Here locally, we always work with absolute path, unless we're sending them to server
	strRelativeFilepath, err := filepath.Rel(configuration.RootDir, strAbsoluteFilepath)
	fmt.Println("Updated file:", strRelativeFilepath)
	// if err != nil {
	// log.Fatal("Could not resolve relative path of ", configuration.RootDir, strAbsoluteFilepath, err)
	// }

	defer conn.Close()
	// Hashing file on our end
	var arrBlockHash []hasher.BlockHash
	arrBlockHash = hasher.HashFile(strAbsoluteFilepath)
	// Sending result to server for update
	fmt.Println("1. Sending to server...")
	fmt.Println(arrBlockHash)
	encoder := gob.NewEncoder(conn)
	err = encoder.Encode(FileHashResult{StrRelativeFilepath: filepath.ToSlash(strRelativeFilepath), ArrBlockHash: arrBlockHash, IsClientUpdate: true})
	if err != nil {
		log.Fatal("Connection error from client (sendclientchanges/sending result): ", err)
	}

	// Receive list of differences from server
	arrFileChange := &FileChangeList{}
	decoder := gob.NewDecoder(conn)
	err = decoder.Decode(arrFileChange)
	if err != nil {
		log.Fatal("Connection error from client (sendclientchanges/receiving diff): ", err)
	}
	fmt.Println("3. received from server")
	fmt.Println(*arrFileChange)
	hasher.UpdateDeltaData(arrFileChange.ArrFileChange, strAbsoluteFilepath)
	// Resending updated data
	err = encoder.Encode(arrFileChange.ArrFileChange)
	if err != nil {
		log.Fatal("Connection error from client (sendclientchanges/resending updated data): ", err)
	}
	fmt.Println("4. Resent to server")
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
		// 2. Get file update
		err = decoder.Decode(fileHashResult)
		if err != nil {
			log.Fatal("Connection error from client (get file update): ", err)
		}
		fmt.Println("2. Receiving from server")
		fmt.Println(*fileHashResult)

		strAbsoluteFilepath := configuration.RootDir + filepath.FromSlash(fileHashResult.StrRelativeFilepath)

		// Check if file exists, if not create it
		// TODO: Possible optimization here, skip all processes just upload it
		if hasher.CheckFileExists(strAbsoluteFilepath) != true {
			fmt.Println("Error while creating local file, aborting update from client", strAbsoluteFilepath)
		}

		// We do our hashing
		fmt.Println("Do hashing from client")
		var arrBlockHash []hasher.BlockHash
		arrBlockHash = hasher.HashFile(strAbsoluteFilepath)
		fmt.Println(arrBlockHash)

		// Compare two files
		var arrFileChange []hasher.FileChange
		arrFileChange = hasher.CompareFileHashes(fileHashResult.ArrBlockHash, arrBlockHash)
		fmt.Printf("We found %d changes!\n", len(arrFileChange))

		// 3. Send difference data from client
		fmt.Println("3. Sending arrFileChange", arrFileChange)
		err = encoder.Encode(FileChangeList{ArrFileChange: arrFileChange})
		if err != nil {
			log.Fatal("Connection error from client (get diff data): ", err)
		}

		// 6. Receive updated differences from server
		arrFileChangeList := &FileChangeList{}
		err = decoder.Decode(arrFileChangeList)
		if err != nil {
			log.Fatal("Connection error from client (received updated diff): ", err)
		}
		fmt.Println("6. decoded")
		fmt.Println(arrFileChangeList)

		// Update destination file
		hasher.UpdateDestinationFile(arrFileChangeList.ArrFileChange, strAbsoluteFilepath)
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

	connreceiver := connectToServer(configuration.CertFilepath, configuration.ServerIp)
	go receiveServerChanges(connreceiver)
	monitorLocalChanges(configuration.RootDir, configuration.CertFilepath, configuration.ServerIp)

	//conn.Close()
	<-done
	fmt.Println("done")
}
