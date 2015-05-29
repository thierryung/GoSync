package hasher

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

const (
	HASH_PRIME_ROOT uint64 = 31
	HASH_MODULO     uint64 = 1024

	// HASH_WINDOW_SIZE int    = 1031
	// HASH_MASK        uint64 = (1 << 19) - 1

	HASH_WINDOW_SIZE int    = 3
	HASH_MASK        uint64 = (1 << 2) - 1

	PROCESSING_DIR string = ".apesync"
)

func UpdateDeltaData(arrFileChange []FileChange, strFilepath string) {
	f, err := os.Open(strFilepath)
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
// TODO: Change .tmp extension to avoid possible collision (random?)
func UpdateDestinationFile(arrFileChange []FileChange, strFilepath string, rootDir string) {
	// TODO: Check if we need to manually split chunks of data read

	var iToRead, iLastFilePointerPosition int = 0, 0
	var strProcessingDir = rootDir + PROCESSING_DIR + string(filepath.Separator)
	var strTempFilepath = strProcessingDir + filepath.Base(strFilepath)

	// open input file
	fi, err := os.Open(strFilepath)
	if err != nil {
		fmt.Println("Error when opening input file ", err)
		return
	}
	// make a read buffer
	r := bufio.NewReader(fi)

	// open temp output file
	// Check if file exists, if not create it
	// TODO: Possible optimization here, skip all processes just upload it
	if CreateFileIfNotExists(strTempFilepath) != true {
		fmt.Println("Error while creating local file, aborting update dest file", strTempFilepath)
		return
	}
	fo, err := os.Create(strTempFilepath)
	if err != nil {
		fmt.Println("Error when opening output file ", err)
		return
	}
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
			fmt.Println("Error when writing output file ", err)
			break
		}

		// Process the add
		if _, err := w.Write(fileChange.DataToAdd); err != nil {
			fmt.Println("Error when writing output file (add) ", err)
			return
		}

		// Process the remove (skip next x bytes)
		// For now we're "reading" bytes to move file pointer, as Seek is not supported by bufio
		buf = make([]byte, fileChange.LengthToRemove)
		_, err = r.Read(buf)
		if err != nil {
			fmt.Println("Error when writing output file (remove) ", err)
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
			fmt.Println("Error when reading last chunk ", err)
			break
		}
		if n == 0 {
			break
		}

		// write a chunk
		if _, err := w.Write(buf[:n]); err != nil {
			fmt.Println("Error when writing last chunk ", err)
			break
		}
	}

	fmt.Println("Flushing file")
	if err = w.Flush(); err != nil {
		fmt.Println("Error when flushing file ", err)
		return
	}

	if err := fi.Close(); err != nil {
		fmt.Println("Error when closing input file ", err)
		return
	}

	if err := fo.Close(); err != nil {
		fmt.Println("Error when closing output file ", err)
		return
	}

	// Delete possible temporary file
	os.Remove(strTempFilepath + ".orig")

	// Renaming old file
	if err = os.Rename(strFilepath, strTempFilepath+".orig"); err != nil {
		fmt.Println("Error when renaming old file ", err)
		return
	}

	// Renaming new file
	if err = os.Rename(strTempFilepath, strFilepath); err != nil {
		fmt.Println("Error when renaming new file ", err)
		return
	}

	// Finally, if all went well, remove old original file
	// if err = os.Remove(strTempFilepath + ".orig"); err != nil {
	// fmt.Println("Error when removing file ", err)
	// return
	// }
}

// CreateFileIfNotExists will create file if it does not already exists
// It will also create associated parent directories if needed
func CreateFileIfNotExists(strFilepath string) bool {
	// Create dir if does not exists
	strDir := filepath.Dir(strFilepath)
	if _, err := os.Stat(strDir); err != nil {
		err := os.MkdirAll(strDir, 0775)
		if err != nil {
			fmt.Println("Error while dir for file", strDir, strFilepath, err)
			return false
		}
	}

	if _, err := os.Stat(strFilepath); err == nil {
		return true
	}
	fmt.Println("No such file or directory, creating...", strFilepath)
	err := ioutil.WriteFile(strFilepath, nil, 0644)
	if err != nil {
		fmt.Println("Error while creating", strFilepath, err)
		return false
	}
	return true
}

//
func PrepareProcessingDir(strDirpath string) bool {
	// Create dir if does not exists
	strDir := strDirpath + PROCESSING_DIR
	if _, err := os.Stat(strDir); err != nil {
		err := os.MkdirAll(strDir, 0775)
		if err != nil {
			fmt.Println("Error while dir for file", strDir, err)
			return false
		}
		fmt.Println("Created processing dir", strDir)
	}
	return true
}
