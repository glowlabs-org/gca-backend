package client

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glowlabs-org/gca-backend/glow"
)

func TestClientHistory(t *testing.T) {
	c, _, _, err := FullClientTestEnvironment(t.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err := c.Close()
		if err != nil {
			t.Error(err)
		}
	}()

	// Initial check: All entries should be empty.
	for i := uint32(0); i < 100; i++ {
		amt, err := c.staticLoadReading(i)
		if err != nil {
			t.Fatal(err)
		}
		if amt != 0 {
			t.Fatal("Expected 0, got", amt)
		}
	}

	// Test saving and loading a reading.
	err = c.staticSaveReading(5, 500)
	if err != nil {
		t.Fatal(err)
	}

	// Verify the saved reading.
	amt, err := c.staticLoadReading(5)
	if err != nil {
		t.Fatal(err)
	}
	if amt != 500 {
		t.Fatal("Expected 500, got", amt)
	}

	// Test saving the same reading twice.
	err = c.staticSaveReading(5, 500)
	if err != nil {
		t.Fatal("Saving the same reading twice should not result in error")
	}

	// Test saving a different reading for the same timesot.
	err = c.staticSaveReading(5, 501)
	if err == nil {
		t.Fatal("bad")
	}
	// What we load should not have changed.
	amt, err = c.staticLoadReading(5)
	if err != nil {
		t.Fatal(err)
	}
	if amt != 500 {
		t.Fatal("Expected 500, got", amt)
	}

	// Test saving a different reading in the same timeslot.
	err = c.staticSaveReading(5, 400)
	if err == nil {
		t.Fatal("Expected error when saving a different reading in the same timeslot")
	}

	// Test reading from an uninitialized timeslot.
	amt, err = c.staticLoadReading(99)
	if err != nil || amt != 0 {
		t.Fatal("Expected 0, got", amt, "with error", err)
	}

	// Saving and loading multiple readings.
	for i := uint32(10); i < 15; i++ {
		err = c.staticSaveReading(i, i*100)
		if err != nil {
			t.Fatal(err)
		}

		amt, err = c.staticLoadReading(i)
		if err != nil || amt != i*100 {
			t.Fatal("Expected", i*100, "got", amt, "with error", err)
		}
	}

	// Final check: Verify that all non-tested entries are still empty.
	for i := uint32(0); i < 100; i++ {
		if i >= 10 && i < 15 || i == 5 {
			continue
		}
		amt, err := c.staticLoadReading(i)
		if err != nil {
			t.Fatal(err)
		}
		if amt != 0 {
			t.Fatal("Expected 0, got", amt)
		}
	}

	// Do the same test, but now with a Client that was created at a future
	// timeslot, so that it's possible to request data that doesn't exist.
	glow.SetCurrentTimeslot(25)
	defer func() {
		glow.SetCurrentTimeslot(0)
	}()
	c2, _, _, err := FullClientTestEnvironment(t.Name() + "_c2")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err := c2.Close()
		if err != nil {
			t.Error(err)
		}
	}()

	// Initial check: All entries should be empty.
	for i := uint32(0); i < 100; i++ {
		amt, err := c2.staticLoadReading(i)
		if err != nil {
			t.Fatal(err)
		}
		if amt != 0 {
			t.Fatal("Expected 0, got", amt)
		}
	}

	// Should get an error when trying to write to timeslot '5', as the
	// file should be starting at slot 25.
	err = c2.staticSaveReading(5, 500)
	if err == nil {
		t.Fatal("bad")
	}

	// Initial check: All entries should be empty.
	for i := uint32(0); i < 100; i++ {
		amt, err := c2.staticLoadReading(i)
		if err != nil {
			t.Fatal(err)
		}
		if amt != 0 {
			t.Fatal("Expected 0, got", amt)
		}
	}

	// Should get an error when trying to write to timeslot '5', as the
	// file should be starting at slot 25.
	err = c2.staticSaveReading(25, 510)
	if err != nil {
		t.Fatal("bad")
	}
	amt, err = c2.staticLoadReading(25)
	if err != nil {
		t.Fatal("bad")
	}
	if amt != 510 {
		t.Fatal("bad")
	}

	// Event logging output
	path := filepath.Join(c.staticBaseDir, "status.txt")
	os.WriteFile(path, []byte(c.DumpStatus()), 0644)
}
