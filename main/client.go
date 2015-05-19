package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"log"
	//"time"
	//"net"

	"thierry/sync/hasher"
)

type FileHashResult struct {
	FileHashParam hasher.FileHashParam
	ArrBlockHash  []hasher.BlockHash
}

type FileChangeList struct {
	ArrFileChange []hasher.FileChange
}

/* func clientreceiver(conn net.Conn) {
    fmt.Println("in receiver")
    p := &P{}
    dec := gob.NewDecoder(conn)
    for {
      dec.Decode(p)
      fmt.Println(*p)
    }
} */

func main() {
	fmt.Println("Starting client...")

	// Init connection with global CA
	certs := x509.NewCertPool()
	pemData, err := ioutil.ReadFile("../cert/capem.pem")
	if err != nil {
		log.Fatal("Connection error from client: ", err)
		return
	}
	certs.AppendCertsFromPEM(pemData)
	config := tls.Config{RootCAs: certs}

	conn, err := tls.Dial("tcp", "192.168.216.128:8080", &config)
	if err != nil {
		log.Fatal("Connection error from client: ", err)
		return
	}
	fmt.Println("Connected to server")

	// Hashing file on our end
	var strFilepath string = "/home/thierry/projects/volclient.txt"
	var arrBlockHash []hasher.BlockHash
	var fileHashParam hasher.FileHashParam
	fileHashParam = hasher.FileHashParam{Filepath: strFilepath}
	arrBlockHash = hasher.HashFile(fileHashParam)
	// Sending result to server for update
	encoder := gob.NewEncoder(conn)
	err = encoder.Encode(FileHashResult{FileHashParam: hasher.FileHashParam{Filepath: strFilepath}, ArrBlockHash: arrBlockHash})
	if err != nil {
		log.Fatal("Connection error from client: ", err)
	}
	fmt.Println("Sending to server...")
	fmt.Println(arrBlockHash)

	// Receive list of differences from server
	arrFileChange := &FileChangeList{}
	decoder := gob.NewDecoder(conn)
	err = decoder.Decode(arrFileChange)
	if err != nil {
		log.Fatal("Connection error from client: ", err)
	}
	fmt.Println("received from server")
	fmt.Println(*arrFileChange)
	hasher.UpdateDeltaData(arrFileChange.ArrFileChange, fileHashParam)
	// Resending updated data
	err = encoder.Encode(arrFileChange.ArrFileChange)
	if err != nil {
		log.Fatal("Connection error from client: ", err)
	}
	fmt.Println("Resent to server")
	fmt.Println(*arrFileChange)

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
	conn.Close()
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
