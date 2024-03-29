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

	"gopkg.in/fsnotify.v1"
	"thierry/sync/hasher"
	"thierry/sync/watch"
)

// Check if we can clean up the following
var done chan bool = make(chan bool)
var configuration Configuration

type FileHashResult struct {
	StrRelativeFilepath string
	ArrBlockHash        []hasher.BlockHash
	UpdaterClientId     string
	// Indicate if this is a client update request
	IsClientUpdate bool
	// Indicates if this is a delete file/folder request
	IsDelete bool
}

type FileChangeList struct {
	ArrFileChange []hasher.FileChange
}

type Configuration struct {
	RootDir      string
	CertFilepath string
	ServerIp     string
}

type FileInProcess struct {
	filepath string
	status   map[string]bool
}

type ListFileInProcess struct {
	list map[string]FileInProcess
}

func (l *ListFileInProcess) setInProcess(filepath string, status string) {
	if _, ok := l.list[filepath]; ok == true {
		fmt.Println("File already in process, trying to reset it to ", filepath, status)
	} else {
		l.list[filepath] = FileInProcess{filepath: filepath, status: make(map[string]bool)}
	}

	l.list[filepath].status[status] = true
	// Status has some extra notifications we want to skip
	if status == "update" {
		//l.list[filepath].status["rename"] = true
		l.list[filepath].status["delete"] = true
	}
}

func (l *ListFileInProcess) setDoneProcess(filepath string, status string) {
	if _, ok := l.list[filepath]; ok == false {
		fmt.Println("Could not find file already in process, trying to set done to ", filepath, status)
		return
	}

	delete(l.list[filepath].status, status)
	fmt.Println("Done with process on file ", filepath, status, l.list[filepath])
}

func (l *ListFileInProcess) isInProcess(filepath string) bool {
	if _, ok := l.list[filepath]; ok == false {
		fmt.Println("Could not find file already in process, trying to check in process status ", filepath)
		return false
	}

	fmt.Println("File check requested, result ", filepath, l.list[filepath].status)
	return len(l.list[filepath].status) > 0
}

// monitorLocalChanges will wait for io changes......
func monitorLocalChanges(rootdir string, cafile string, server string, listFileInProcess *ListFileInProcess) {
	fmt.Println("*** Recursively monitoring folder", rootdir)
	watcher, err := watch.NewWatcher(rootdir, hasher.PROCESSING_DIR)
	//watcher, err := watch.NewRecursiveWatcher(rootdir, hasher.PROCESSING_DIR)
	if err != nil {
		log.Println("Watcher create error : ", err)
	}
	defer watcher.Close()
	_done := make(chan bool)

	go func() {
		watcher.start()

		for {
			select {
			case event := <-watcher.Events:
				switch {
				case event.Op&fsnotify.Create == fsnotify.Create:
					fi, err := os.Stat(event.Name)
					if err != nil {
						// eg. stat .subl513.tmp : no such file or directory
						fmt.Println(err)
						continue
					} else if fi.IsDir() {
						fmt.Println("Detected new directory", event.Name)
						if !watch.ShouldIgnoreFile(filepath.Base(event.Name), hasher.PROCESSING_DIR) {
							fmt.Println("Monitoring new folder...")
							watcher.AddFolder(event.Name)
							connsender := connectToServer(cafile, server)
							go sendClientFolderChanges(connsender, event.Name, listFileInProcess)
							//watcher.Folders <- event.Name
						}
					} else {
						fmt.Println("Detected new file, for now do nothing", event.Name)
						// watcher.Files <- event.Name // created a file
						// TODO
					}

				case event.Op&fsnotify.Write == fsnotify.Write:
					// modified a file, assuming that you don't modify folders
					fmt.Println("Detected file modification", event.Name)
					// Don't handle folder change, since they receive notification
					// when a file they contain is changed
					fi, err := os.Stat(event.Name)
					if err != nil {
						fmt.Println(err)
						continue
					}
					if fi.Mode().IsRegular() {
						// watcher.Files <- event.Name
						log.Println("Modified file: ", event.Name)
						// connsender := connectToServer(cafile, server)
						// go sendClientChanges(connsender, event.Name, listFileInProcess)
					}
				case event.Op&fsnotify.Remove == fsnotify.Remove:
					log.Println("Removed file: ", event.Name)
					// connsender := connectToServer(cafile, server)
					// go sendClientDelete(connsender, event.Name, listFileInProcess)
				case event.Op&fsnotify.Rename == fsnotify.Rename:
					log.Println("Renamed file: ", event.Name)
					// The following is to handle an issue in fsnotify
					// On rename, fsnotify sends three events on linux: RENAME(old), CREATE(new), RENAME(new)
					// fsnotify sends two events on windows: RENAME(old), CREATE(new)
					// The way we handle this is:
					// 1. If there is a second rename, skip it
					// 2. When the first rename happens, remove old file/folder
					// 3. We'll re-add it when the new create comes in
					// Step 2 and 3 might be optimized later by remembering which was old/new and performing simple move
					_, err := os.Stat(event.Name)
					if err != nil {
						// Rename talks about a file/folder now gone, send a remove request to server
						log.Println("Rename leading to delete", event.Name)
						connsender := connectToServer(cafile, server)
						go sendClientDelete(connsender, event.Name, listFileInProcess)
					} else {
						// Rename talks about a file/folder already existing, skip it (do nothing)
					}
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

func sendClientChanges(conn net.Conn, strAbsoluteFilepath string, listFileInProcess *ListFileInProcess) {
	// Here locally, we always work with absolute path, unless we're sending them to server
	strRelativeFilepath, err := filepath.Rel(configuration.RootDir, strAbsoluteFilepath)
	fmt.Println("Updated file:", strRelativeFilepath)
	if err != nil {
		log.Fatal("Could not resolve relative path of ", configuration.RootDir, strAbsoluteFilepath, err)
	}

	// Check if we're currently working on this file (fsnotify windows may send double notices)
	defer listFileInProcess.setDoneProcess(strRelativeFilepath, "update")
	if listFileInProcess.isInProcess(strRelativeFilepath) {
		fmt.Println("Currently working on file, skipping...", strAbsoluteFilepath)
		return
	}
	listFileInProcess.setInProcess(strRelativeFilepath, "update")

	defer conn.Close()
	// Hashing file on our end
	var arrBlockHash []hasher.BlockHash
	arrBlockHash = hasher.HashFile(strAbsoluteFilepath)
	// Sending result to server for update
	fmt.Println("1. Sending update to server...")
	fmt.Println(FileHashResult{StrRelativeFilepath: filepath.ToSlash(strRelativeFilepath), ArrBlockHash: arrBlockHash, IsClientUpdate: true})
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

	fmt.Println("Done processing file change", strAbsoluteFilepath)
}

// sendClientFolderChanges looks into the requested folder and for each file
// will send a "new file" request to server
func sendClientFolderChanges(conn net.Conn, strAbsoluteFilepath string, listFileInProcess *ListFileInProcess) {
	fi, err := os.Stat(strAbsoluteFilepath)
	if err != nil {
		fmt.Println("Not a real dir!", strAbsoluteFilepath)
		return
	}

	fileList := []string{}
	err = filepath.Walk(strAbsoluteFilepath, func(path string, f os.FileInfo, err error) error {
		fileList = append(fileList, path)
		return nil
	})

	if err != nil {
		fmt.Println("Error when walking through new directory", strAbsoluteFilepath)
	}

	// Only send files through
	for _, file := range fileList {
		fi, err = os.Stat(file)
		if err != nil {
			fmt.Println(err)
			continue
		}
		if fi.Mode().IsRegular() {
			fmt.Println(file)
			//sendClientChanges(conn, file, listFileInProcess)
		}
	}
}

func sendClientDelete(conn net.Conn, strAbsoluteFilepath string, listFileInProcess *ListFileInProcess) {
	// Here locally, we always work with absolute path, unless we're sending them to server
	strRelativeFilepath, err := filepath.Rel(configuration.RootDir, strAbsoluteFilepath)
	fmt.Println("Deleted file:", strRelativeFilepath)

	// Check if we're currently working on this file (fsnotify windows may send double notices)
	defer listFileInProcess.setDoneProcess(strRelativeFilepath, "delete")
	if listFileInProcess.isInProcess(strRelativeFilepath) {
		fmt.Println("Currently working on file, skipping...", strAbsoluteFilepath)
		return
	}
	listFileInProcess.setInProcess(strRelativeFilepath, "delete")

	defer conn.Close()
	// Sending result to server for delete
	fmt.Println("1. Sending delete to server...")
	encoder := gob.NewEncoder(conn)
	err = encoder.Encode(FileHashResult{StrRelativeFilepath: filepath.ToSlash(strRelativeFilepath), IsClientUpdate: true, IsDelete: true})
	if err != nil {
		log.Fatal("Connection error from client (sendClientDelete/sending delete): ", err)
	}

	// Indicate we're done working on this file
	fmt.Println("Done processing file delete", strAbsoluteFilepath)
}

func receiveServerChanges(conn net.Conn, listFileInProcess *ListFileInProcess) {
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

		// Check if we're trying to update or delete
		if fileHashResult.IsDelete == true {
			listFileInProcess.setInProcess(fileHashResult.StrRelativeFilepath, "delete")
			processDeleteFromServer(fileHashResult)
			listFileInProcess.setDoneProcess(fileHashResult.StrRelativeFilepath, "delete")
		} else {
			listFileInProcess.setInProcess(fileHashResult.StrRelativeFilepath, "update")
			processChangeFromServer(fileHashResult, encoder, decoder)
			listFileInProcess.setDoneProcess(fileHashResult.StrRelativeFilepath, "update")
		}

		fmt.Println("Done processing from server", fileHashResult.StrRelativeFilepath)
	}
}

func processChangeFromServer(fileHashResult *FileHashResult, encoder *gob.Encoder, decoder *gob.Decoder) {
	strAbsoluteFilepath := configuration.RootDir + filepath.FromSlash(fileHashResult.StrRelativeFilepath)

	// Check if file exists, if not create it
	// TODO: Possible optimization here, skip all processes just upload it
	if hasher.CreateFileIfNotExists(strAbsoluteFilepath) != true {
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
	err := encoder.Encode(FileChangeList{ArrFileChange: arrFileChange})
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
	hasher.UpdateDestinationFile(arrFileChangeList.ArrFileChange, strAbsoluteFilepath, configuration.RootDir)
}

func processDeleteFromServer(fileHashResult *FileHashResult) {
	strAbsoluteFilepath := configuration.RootDir + filepath.FromSlash(fileHashResult.StrRelativeFilepath)

	// Check if file exists, if not skip it all
	if _, err := os.Stat(strAbsoluteFilepath); err != nil {
		fmt.Println("File to delete from client doesn't exist, skip it", strAbsoluteFilepath)
		return
	}

	// Delete file on local
	fmt.Println("Removing all under", strAbsoluteFilepath)
	err := os.RemoveAll(strAbsoluteFilepath)
	if err != nil {
		fmt.Println("There was an error removing...", strAbsoluteFilepath)
	}
}

func connectToServer(cafile string, server string) net.Conn {
	// Init connection with global CA
	certs := x509.NewCertPool()
	pemData, err := ioutil.ReadFile(cafile)
	if err != nil {
		log.Fatal("Connection error from client file not found: ", err, cafile)
		return nil
	}
	certs.AppendCertsFromPEM(pemData)
	config := tls.Config{RootCAs: certs}

	conn, err := tls.Dial("tcp", server, &config)
	if err != nil {
		log.Fatal("Connection error from client when dialing: ", err, certs)
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

	// List of currently modifying files. To avoid leaking recursive notification.
	// And also to fix issues such as https://github.com/howeyc/fsnotify/issues/106
	listFileInProcess := &ListFileInProcess{make(map[string]FileInProcess)}
	fmt.Println(configuration.CertFilepath)

	connreceiver := connectToServer(configuration.CertFilepath, configuration.ServerIp)
	go receiveServerChanges(connreceiver, listFileInProcess)
	monitorLocalChanges(configuration.RootDir, configuration.CertFilepath, configuration.ServerIp, listFileInProcess)

	//conn.Close()
	<-done
	fmt.Println("done")
}