package client

// history.go contains all of the logic for reading from the history file,
// which contains all of the historic readings from the client that indicate
// how much power a solar panel has produced.
//
// The file is structured so that the first 4 bytes indicate what timeslot the
// file starts recording data at. Each following 4 bytes contains the number of
// watt hours that was produced by the solar panel each timeslot.

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// loadHistoryFile will open the history file, save the handle in the client,
// and determine the timeslot offset of the history file.
func (c *Client) loadHistory() error {
	// Open the history file and make it part of the client.
	path := filepath.Join(c.staticBaseDir, HistoryFile)
	f, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("unable to open history file: %v", err)
	}
	c.staticHistoryFile = f

	// Read the offset bytes.
	var offsetBytes [4]byte
	_, err = f.ReadAt(offsetBytes[:], 0)
	if err != nil {
		return fmt.Errorf("unable to read offset bytes: %v", err)
	}

	// Convert to uint32
	c.staticHistoryOffset = binary.LittleEndian.Uint32(offsetBytes[:])
	return nil
}

// staticSaveReading will save a particular reading to the file, returning an
// error if a reading already exists in that timeslot which does not match the
// provided reading.
func (c *Client) staticSaveReading(timeslot uint32, reading uint32) error {
	// Check that the reading is in-bounds on the file.
	if timeslot < c.staticHistoryOffset {
		return fmt.Errorf("cannot save a reading for a timeslot that predates the history file genesis")
	}
	byteOffset := 4 * (1 + timeslot - c.staticHistoryOffset)

	// Load the reading from this offset.
	current, err := c.staticLoadReading(timeslot)
	if err != nil {
		return fmt.Errorf("unable to load reading for this timeslot: %v", err)
	}
	// If the reading has already been saved, this is a no-op.
	if current == reading {
		return nil
	}
	if current != 0 {
		return fmt.Errorf("unable to save reading because we already have a different reading for this timeslot")
	}

	// Write the reading to the file.
	var data [4]byte
	binary.LittleEndian.PutUint32(data[:], reading)
	_, err = c.staticHistoryFile.WriteAt(data[:], int64(byteOffset))
	if err != nil {
		return fmt.Errorf("unable to write the reading: %v", err)
	}
	return nil
}

// staticLoadReading will load a reading from the provided timelsot, returning
// an error if it is out of bounds. If no reading exists, '0' will be returned.
func (c *Client) staticLoadReading(timeslot uint32) (uint32, error) {
	if timeslot < c.staticHistoryOffset {
		// This timeslot doesn't have data, so the natural response is
		// nothing.
		return 0, nil
	}

	// Read from the file.
	var data [4]byte
	byteOffset := 4 * (1 + timeslot - c.staticHistoryOffset)
	_, err := c.staticHistoryFile.ReadAt(data[:], int64(byteOffset))
	if err == io.EOF {
		// If the timeslot is for a part of the file that was never
		// written, the natural response is nothing.
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("cannot load a reading: %v", err)
	}

	return binary.LittleEndian.Uint32(data[:]), nil
}
