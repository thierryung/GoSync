package hasher

// Represents a change between source and dest file
type FileChange struct {
	dataToAdd []byte
	// TODO: Check if int is enough (overflow)
	positionInSourceFile int
	// TODO: Check if int is enough (overflow)
	lengthToAdd int
	// TODO: Check if int is enough (overflow)
	lengthToRemove int
}

// TODO: Handle logic when change is at the very beginning or at the very end
// TODO: Make a prettier loop (loop ending i++ j++)
// TODO: Free array hash map (or check memory for every init)
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
			mapHashSource[arrHashSource[i].hash] = i
			mapHashDest[arrHashDest[j].hash] = j

			// Check if any past hash matches one of our current hash (see algo)
			iPosMatchSource, okSource := mapHashSource[arrHashDest[j].hash]
			iPosMatchDest, okDest := mapHashDest[arrHashSource[i].hash]
			// In which case, we're now simply exchanging data (add/remove)
			if okSource {
				arrFileChange = append(arrFileChange, FileChange{lengthToAdd: CalculateLengthBetween(arrHashSource, iHashPosSource, iPosMatchSource), positionInSourceFile: arrHashSource[iHashPosSource].positionInFile, lengthToRemove: CalculateLengthBetween(arrHashDest, iHashPosDest, j)})

				// We're done with checking diff, go back to standard loop
				i = iPosMatchSource
				bIsCheckingDiff = false

			} else if okDest {
				arrFileChange = append(arrFileChange, FileChange{lengthToAdd: CalculateLengthBetween(arrHashSource, iHashPosSource, i), positionInSourceFile: arrHashSource[iHashPosSource].positionInFile, lengthToRemove: CalculateLengthBetween(arrHashDest, iHashPosDest, iPosMatchDest)})

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
		if arrHashSource[i].hash == arrHashDest[j].hash {
			i++
			j++

		} else {
			// Non matching data, remember current state
			bIsCheckingDiff = true
			iHashPosSource = i
			iHashPosDest = j
			// Initialize our map array
			mapHashSource = make(map[[16]byte]int)
			mapHashSource[arrHashSource[i].hash] = i
			mapHashDest = make(map[[16]byte]int)
			mapHashDest[arrHashDest[j].hash] = j
			// And go to next
			i++
			j++
		}
	}

	// We're now out of the loop, was the last block different?
	// Or do we have a hash list longer than the other?
	if bIsCheckingDiff || lenSource != lenDest {
		// In this case, we're simply overriding data (removing old, adding new)
		arrFileChange = append(arrFileChange, FileChange{lengthToAdd: CalculateLengthBetween(arrHashSource, i-1, len(arrHashSource)), positionInSourceFile: arrHashSource[i-1].positionInFile, lengthToRemove: CalculateLengthBetween(arrHashDest, j-1, len(arrHashDest))})
	}

	return arrFileChange
}
