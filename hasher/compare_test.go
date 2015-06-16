package hasher

import (
	"testing"
)

func TestCompareFileHashes(t *testing.T) {
	// TODO: Create hash hash, hash dest, then compare
	/* // var strFilepath string = "/home/thierry/projects/vol1"
	// var strFilepath2 string = "/home/thierry/projects/vol2"
	var strFilepath1 string = "volclientlocal.txt"
	var strFilepath2 string = "volserverlocal.txt"

	var arrBlockHash []hasher.BlockHash
	var arrBlockHash2 []hasher.BlockHash
	var arrFileChange []hasher.FileChange

	// Hash file 1
	arrBlockHash = hasher.HashFile(configuration.RootDir + strFilepath1)
	fmt.Println(arrBlockHash)

	// Hash file 2
	arrBlockHash2 = hasher.HashFile(configuration.RootDir + strFilepath2)
	fmt.Println(arrBlockHash2)

	// Compare two files
	arrFileChange = hasher.CompareFileHashes(arrBlockHash, arrBlockHash2)
	fmt.Printf("We found %d changes!\n", len(arrFileChange))
	fmt.Println(arrFileChange)

	// Get difference data
	hasher.UpdateDeltaData(arrFileChange, configuration.RootDir + strFilepath1)
	fmt.Println(arrFileChange)

	// Update destination file
	hasher.UpdateDestinationFile(arrFileChange, configuration.RootDir + strFilepath2)
	os.Exit(0) */
}
