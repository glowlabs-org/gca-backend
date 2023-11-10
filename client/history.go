package client

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
)

// history.go contains all of the logic for reading from the history file,
// which contains all of the historic readings from the monitoring hardware
// that indicate how much power a solar panel has produced.
//
// The file is structured so that the first 4 bytes indicate what timeslot the
// file starts recording data at. Each following 4 bytes contains the number of
// watt hours that was produced by the solar panel each timeslot.

// loadHistoryFile will open the history file, save the handle in the client,
// and determine the timeslot offset of the history file.
func (c *Client) loadHistory() error {
	// Open the history file and make it part of the client.
	path := filepath.Join(c.baseDir, HistoryFile)
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("unable to open history file: %v", err)
	}
	c.historyFile = f

	// Read the offset bytes.
	var offsetBytes [4]byte
	_, err = f.ReadAt(offsetBytes[:], 0)
	if err != nil {
		return fmt.Errorf("unable to read offset bytes: %v", err)
	}

	// Convert to uint32
	c.historyOffset = binary.LittleEndian.Uint32(offsetBytes[:])
	return nil
}

// saveReading will save a particular reading to the file, returning an error
// if a reading already exists in that timeslot which does not match the
// provided reading.
func (c *Client) saveReading(timeslot uint32, reading uint32) error {
	// Check that the reading is in-boudns on the file.
	if timeslot < c.historyOffset {
		return fmt.Errorf("cannot save a reading for a timeslot that predates the history file genesis")
	}
	byteOffset := 4 * (1 + timeslot - c.historyOffset)

	// Load the reading from this offset.
	current, err := c.loadReading(timeslot)
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

	var data [4]byte
	binary.LittleEndian.PutUint32(data[:], reading)
	_, err = c.historyFile.WriteAt(data[:], int64(byteOffset))
	if err != nil {
		return fmt.Errorf("unable to write the reading: %v", err)
	}
	return nil
}

// loadReading will load a reading from the provided timelsot, returning an
// error if it is out of bounds. If no reading exists, '0' will be returned.
func (c *Client) loadReading(timeslot uint32) (uint32, error) {
	if timeslot < c.historyOffset {
		return 0, fmt.Errorf("cannot save a reading for a timeslot that predates the history file genesis")
	}
	byteOffset := 4 * (1 + timeslot - c.historyOffset)

	var data [4]byte
	_, err := c.historyFile.ReadAt(data[:], int64(byteOffset))
	if err != nil {
		return 0, fmt.Errorf("cannot load a reading: %v", err)
	}

	return binary.LittleEndian.Uint32(data[:]), nil
}
