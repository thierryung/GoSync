package hasher

import (
	"bufio"
)

// Represents our circling window
type WindowBytes struct {
	// The current bytes for this window
	currBytes []byte
	// Start index since this is a circling window
	startIndex int
	// Length of bytes used in the circling window, since we may read less than window
	length int
	// The current bytes for this block (meaning at least the size of currBytes)
	currBlock []byte
}

func (w *WindowBytes) init(windowSize int) {
	// Init our window byte
	w.currBytes = make([]byte, windowSize)
	// Init our block array at a larger size
	// It will automatically expand as needed, but having it large enough is better
	w.currBlock = make([]byte, windowSize*10000)
	w.startIndex = 0
}

func (w *WindowBytes) getFirstByte() byte {
	return w.currBytes[w.startIndex]
}

func (w *WindowBytes) getBytes() []byte {
	var windowSize = len(w.currBytes)
	var currBytes = make([]byte, windowSize)
	c := 0

	// Circling window. We start at wherever startIndex is
	// We keep going until we come back to startIndex, or we reached max length
	for i := w.startIndex; (i != w.startIndex || c == 0) && c < w.length; i++ {
		currBytes[c] = w.currBytes[i]
		c++
		if i >= windowSize-1 {
			i = -1
		}
	}
	return currBytes
}

func (w *WindowBytes) addByte(b byte) {
	// Add byte to window
	w.currBytes[w.startIndex] = b
	w.startIndex++
	if w.startIndex >= len(w.currBytes) {
		w.startIndex = 0
	}

	// Add byte to block
	w.currBlock = append(w.currBlock, b)
}

func (w *WindowBytes) readFull(reader *bufio.Reader) (n int, err error) {
	// Reset window bytes
	w.startIndex = 0
	var c int = 0
	var cmax int = len(w.currBytes)
	for c = 0; c < cmax; c++ {
		w.currBytes[c], err = reader.ReadByte()
		if err != nil {
			break
		}
	}
	w.length = c

	// Copy this window to our block
	copy(w.currBlock, w.currBytes)
	// Truncate off the rest
	w.currBlock = w.currBlock[0:w.length]

	return c, err
}
