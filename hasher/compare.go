package hasher

// Represents a change between source and dest file
type FileChange struct {
	DataToAdd []byte
	// TODO: Check if int is enough (overflow)
	PositionInSourceFile int
	// TODO: Check if int is enough (overflow)
	LengthToAdd int
	// TODO: Check if int is enough (overflow)
	LengthToRemove int
}

func checkOutOfBounds(arr []BlockHash, num int) int {
	if num < 0 {
		num = 0
	}
	if num > len(arr) {
		num = len(arr)
	}
	return num
}

func CalculateLengthBetween(b []BlockHash, start, end int) int {
	var result int = 0
	// Sanity check
	if start < 0 {
		start = 0
	}
	if end > len(b) {
		end = len(b)
	}
	for i := start; i < end; i++ {
		result += b[i].Length
	}
	return result
}

// TODO: Make a prettier loop (loop ending i++ j++)
// TODO: Free array hash map (or check memory usage for every init)
// CompareFileHashes will ...
// Also checks for changes at the beginning or end of the blocks
func CompareFileHashes(arrHashSource, arrHashDest []BlockHash) []FileChange {
	var arrFileChange []FileChange
	var i, j int = 0, 0
	var lenSource, lenDest int = len(arrHashSource), len(arrHashDest)
	var bIsCheckingDiff bool = false
	var iHashPosSource, iHashPosDest int
	var mapHashSource, mapHashDest map[[16]byte]int

	// Loop through both arrays, find differences
	for i < lenSource && j < lenDest {
		// Logic for loop while currently checking a diff
		if bIsCheckingDiff {
			mapHashSource[arrHashSource[i].Hash] = i
			mapHashDest[arrHashDest[j].Hash] = j

			// Check if any past hash matches one of our current hash (see algo)
			iPosMatchSource, okSource := mapHashSource[arrHashDest[j].Hash]
			iPosMatchDest, okDest := mapHashDest[arrHashSource[i].Hash]
			// In which case, we're now simply exchanging data (add/remove)
			if okSource {
				arrFileChange = append(arrFileChange, FileChange{LengthToAdd: CalculateLengthBetween(arrHashSource, iHashPosSource, iPosMatchSource), PositionInSourceFile: arrHashSource[iHashPosSource].PositionInFile, LengthToRemove: CalculateLengthBetween(arrHashDest, iHashPosDest, j)})

				// We're done with checking diff, go back to standard loop
				i = iPosMatchSource
				bIsCheckingDiff = false

			} else if okDest {
				arrFileChange = append(arrFileChange, FileChange{LengthToAdd: CalculateLengthBetween(arrHashSource, iHashPosSource, i), PositionInSourceFile: arrHashSource[iHashPosSource].PositionInFile, LengthToRemove: CalculateLengthBetween(arrHashDest, iHashPosDest, iPosMatchDest)})

				// We're done with checking diff, go back to standard loop
				j = iPosMatchDest
				bIsCheckingDiff = false
			}

			// Go to next set
			i++
			j++
			continue
		}

		// Matching data, simply go to next
		iHashPosSource = i
		iHashPosDest = j
		if arrHashSource[i].Hash == arrHashDest[j].Hash {
			i++
			j++

		} else {
			// Non matching data, remember current state
			bIsCheckingDiff = true
			// Initialize our map array
			mapHashSource = make(map[[16]byte]int)
			mapHashSource[arrHashSource[i].Hash] = i
			mapHashDest = make(map[[16]byte]int)
			mapHashDest[arrHashDest[j].Hash] = j
			// And go to next
			i++
			j++
		}
	}

	// We're now out of the loop, was the last block different?
	// Or do we have a hash list longer than the other?
	if bIsCheckingDiff || lenSource != lenDest {
		posSourceFile := 0
		if iHashPosSource < len(arrHashSource) {
			posSourceFile = arrHashSource[iHashPosSource].PositionInFile
		}
		// In this case, we're simply overriding data (removing old, adding new)
		arrFileChange = append(arrFileChange, FileChange{LengthToAdd: CalculateLengthBetween(arrHashSource, iHashPosSource, len(arrHashSource)), PositionInSourceFile: posSourceFile, LengthToRemove: CalculateLengthBetween(arrHashDest, iHashPosDest, len(arrHashDest))})
	}

	return arrFileChange
}
