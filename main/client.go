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
						watcher.Files <- event.Name // created a file
					}

				case event.Op&fsnotify.Write == fsnotify.Write:
					// modified a file, assuming that you don't modify folders
					fmt.Println("Detected file modification %s", event.Name)
					watcher.Files <- event.Name
					log.Println("Modified file: ", event.Name)
					connsender := connectToServer(cafile, server)
					go sendClientChanges(connsender, event.Name)
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

	/*
	   	go func() {
	   		for {
	   			select {
	   			case event := <-watcher.Events:
	   				switch {
	   				case event.Op&fsnotify.Write == fsnotify.Write:
	   					log.Println("Modified file: ", event.Name)
	             connsender := connectToServer(cafile, server)
	             go sendClientChanges(connsender, event.Name)
	   				case event.Op&fsnotify.Create == fsnotify.Create:
	   					log.Println("Created file: ", event.Name)
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

	   	err = watcher.Add("/home/thierry/projects/testdata/")
	   	if err != nil {
	       log.Println("Watcher add error : ", err)
	   	} */

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

	// Get file update
	fileHashResult := &FileHashResult{}
	err = decoder.Decode(fileHashResult)
	if err != nil {
		log.Fatal("Connection error from client (get file update): ", err)
	}
	fmt.Println(*fileHashResult)

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
	err = encoder.Encode(FileChangeList{ArrFileChange: arrFileChange})
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
	//connreceiver := connectToServer("../cert/capem.pem", "192.168.216.128:8080")
	//go receiveServerChanges(connreceiver)
	monitorLocalChanges("../cert/capem.pem", "192.168.216.128:8080")

	// For now, sleep 1 second
	// What we really want to do is to block (channel?)
	// until we have a file change locally to send to server
	time.Sleep(1000 * time.Millisecond)

	/*
		  go clientreceiver(conn)
			encoder := gob.NewEncoder(conn)
			p := P{"testttt"}
			fmt.Println(p)
			encoder.Encode(p)
			time.Sleep(1000 * time.Millisecond)
			p = P{"testttt2"}
			fmt.Println(p)
			encoder.Encode(p)
			encoder.Encode(p)
			encoder.Encode(p)

		  for {
		  }

			// Receive from server into client
			dec := gob.NewDecoder(conn)
			p2 := &P{}
			for dec.Decode(p2) == nil {
				fmt.Println(*p2)
			}
	*/
	//conn.Close()
	<-done
	fmt.Println("done")
}

/* package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

func main() {
	b, err := ioutil.ReadFile("../cert/cert2.pem")
	if err != nil {
		log.Fatal(err)
	}
	pool := x509.NewCertPool()
	if ok := pool.AppendCertsFromPEM(b); !ok {
		log.Fatal("failed to append cert")
	}
	tc := &tls.Config{RootCAs: pool}
	tr := &http.Transport{TLSClientConfig: tc}
	client := &http.Client{Transport: tr}
	req, err := http.NewRequest("GET", "https://127.0.0.1:8080", nil)
	if err != nil {
		log.Fatal(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	b, err = ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(b))
} */

