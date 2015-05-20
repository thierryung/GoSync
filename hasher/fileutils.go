package hasher

import (
	"bufio"
	"fmt"
	"io"
	"os"
)

const (
	HASH_PRIME_ROOT uint64 = 31
	HASH_MODULO     uint64 = 1024

	// HASH_WINDOW_SIZE int    = 1031
	// HASH_MASK        uint64 = (1 << 19) - 1

	HASH_WINDOW_SIZE int    = 3
	HASH_MASK        uint64 = (1 << 2) - 1
)

// Convenient container with params for a file hash
// TODO: Check if now container is necessary
type FileHashParam struct {
	Filepath string
}

func UpdateDeltaData(arrFileChange []FileChange, fileHashParamSource FileHashParam) {
	f, err := os.Open(fileHashParamSource.Filepath)
	if err != nil {
		return
	}
	defer func() {
		if err := f.Close(); err != nil {
			return
		}
	}()
	// Loop through our changes
	for key := range arrFileChange {
		if arrFileChange[key].LengthToAdd <= 0 {
			continue
		}
		// TODO: Better error check, check for offset ok
		_, err := f.Seek(int64(arrFileChange[key].PositionInSourceFile), 0)
		if err != nil {
			return
		}
		newData := make([]byte, arrFileChange[key].LengthToAdd)
		// TODO: Better error check, check for num read ok
		_, err = io.ReadFull(f, newData)
		if err != nil {
			return
		}
		//fmt.Printf("Read from source pos %d: %s\n", arrFileChange[key].PositionInSourceFile, newData)
		arrFileChange[key].DataToAdd = newData
	}
}

// 1. Final data will be created in a temp file
// 2. Original file will be moved
// 3. Temp file renamed to original
// 4. Do some file integrity checking
// 5. If all goes well, remove original
func UpdateDestinationFile(arrFileChange []FileChange, fileHashParamDest FileHashParam) {
	// TODO: Check if we need to manually split chunks of data read

	var iToRead, iLastFilePointerPosition int = 0, 0

	// open input file
	fi, err := os.Open(fileHashParamDest.Filepath)
	if err != nil {
		fmt.Print("Error when opening input file ")
		fmt.Println(err)
		return
	}
	// close fi on exit and check for its returned error
	defer func() {
		if err := fi.Close(); err != nil {
			return
		}
	}()
	// make a read buffer
	r := bufio.NewReader(fi)

	// open output file
	fo, err := os.Create(fileHashParamDest.Filepath + ".tmp")
	if err != nil {
		return
	}
	// close fo on exit and check for its returned error
	defer func() {
		if err := fo.Close(); err != nil {
			return
		}
	}()
	// make a write buffer
	w := bufio.NewWriter(fo)

	// Loop through our changes
	for _, fileChange := range arrFileChange {
		// Read until change position
		iToRead = fileChange.PositionInSourceFile - iLastFilePointerPosition
		buf := make([]byte, iToRead)
		// read a chunk
		// TODO: Check errors
		n, err := r.Read(buf)
		if err != nil {
			break
		}
		// Write data up until position of change
		if _, err := w.Write(buf[:n]); err != nil {
			break
		}

		// Process the add
		if _, err := w.Write(fileChange.DataToAdd); err != nil {
			return
		}

		// Process the remove (skip next x bytes)
		// For now we're "reading" bytes to move file pointer, as Seek is not supported by bufio
		buf = make([]byte, fileChange.LengthToRemove)
		_, err = r.Read(buf)
		if err != nil {
			return
		}

		// Update our file pointer position
		iLastFilePointerPosition = fileChange.PositionInSourceFile + fileChange.LengthToAdd
	}

	// Write last chunk
	buf := make([]byte, 1024)
	for {
		// read a chunk
		n, err := r.Read(buf)
		if err != nil && err != io.EOF {
			break
		}
		if n == 0 {
			break
		}

		// write a chunk
		if _, err := w.Write(buf[:n]); err != nil {
			break
		}
	}

	if err = w.Flush(); err != nil {
		return
	}
}
