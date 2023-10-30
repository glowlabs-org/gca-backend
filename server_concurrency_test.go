package main

// This file contains one large integration test focused on detecting race
// conditions. It's intentionally a pretty chaotic test that has every API
// being accessed rapidly, simultaneously. It's not worried so much about the
// correctness of the logic as it is worried about ensuring that each piece is
// able to operate independently and maintain a level of consistency, all
// without setting off the race detector.

import (
	"sync/atomic"
	"testing"
	"time"
)

// TestConcurrency is a large integration test that tries to get actions firing
// on all APIs of the server simultanously while the race detector runs, to
// determine whether there are any race conditions at play.
//
// One of the major points of this test in particular is to run all the code
// "by the book" - we try as much as possible to avoid referencing the internal
// state of the gcaServer and instead just query its APIs.
func TestConcurrency(t *testing.T) {
	// This test adjusts the flow of time. Defer a statement that resets
	// the time for future tests.
	defer func() {
		setCurrentTimeslot(0)
	}()

	// The concurrency test starts at time 0 with no GCA key provided, only
	// the temp key is in place. This means that all of the concurrent
	// operations will be failing, but that's okay. We want to make sure
	// everything is failing, and that there are no race conditions.
	//
	// That means we start with a server that has no temp key.
	dir := generateTestDir(t.Name())
	gcas, tempPrivKey, err := gcaServerWithTempKey(dir)
	if err != nil {
		t.Fatal(err)
	}

	stopSignal := make(chan struct{})

	// Create a few threads that will be repeatedly submitting GCA keys
	// with the wrong signature. Every try should fail.
	for i := 0; i < 3; i++ {
		// Create the bad key.
		badKey := tempPrivKey
		badKey[0] += byte(i + 1)

		// Create a goroutine to repeatedly submit the bad key .
		go func(key PrivateKey) {
			// Try until the stop signal is sent.
			i := 0
			for {
				// Try submitting the key.
				_, err := gcas.submitGCAKey(key)
				if err == nil {
					t.Fatal("should not be able to submit a GCA key with a bad private key")
				}

				// Check for the stop signal.
				select {
				case <-stopSignal:
					return
				default:
				}

				// Wait 10 milliseconds between every 5th
				// attempt to minimize cpu spam.
				if i%5 == 0 {
					time.Sleep(5 * time.Millisecond)
				}
				i++
			}
		}(badKey)
	}

	// Create a few threads that will be repeatedly attempting to authorize
	// equipment using a bad key. Every try should fail.
	for i := 0; i < 3; i++ {
		// Create the bad key.
		badKey := tempPrivKey
		badKey[0] += byte(i + 1)

		// Create a goroutine to repeatedly submit the bad key .
		go func(key PrivateKey) {
			// Try until the stop signal is sent.
			i := 0
			for {
				// Try submitting some new hardware.
				_, _, err := gcas.submitNewHardware(666777, key)
				if err == nil {
					t.Fatal("should not be able to submit a GCA key with a bad private key")
				}

				// Check for the stop signal.
				select {
				case <-stopSignal:
					return
				default:
				}

				// Wait 10 milliseconds between every 5th
				// attempt to minimize cpu spam.
				if i%5 == 0 {
					time.Sleep(5 * time.Millisecond)
				}
				i++
			}
		}(badKey)
	}

	// Create a few threads that will be repeatedly sending bad reports
	// from equipment that was never authorized.
	for i := 0; i < 3; i++ {
		// Create a real keypair for the equipment, then create the
		// equipment. The Signature is left blank because no valid
		// signature auth can exist yet, as we haven't generated the
		// GCA key yet.
		ePub, ePriv := GenerateKeyPair()
		ea := EquipmentAuthorization{
			ShortID:    5,
			PublicKey:  ePub,
			Capacity:   15e9,
			Debt:       5e6,
			Expiration: 15e6,
		}
		go func(ea EquipmentAuthorization, ePriv PrivateKey) {
			// Try until the stop signal is sent.
			i := 0
			for {
				// Try submitting a new report.
				err := gcas.sendEquipmentReport(ea, ePriv)
				if err != nil {
					t.Fatal("even though the reports are invalid, they should still send correctly over UDP")
				}

				// Check for the stop signal.
				select {
				case <-stopSignal:
					return
				default:
				}

				// Wait 10 milliseconds between every 5th
				// attempt to minimize cpu spam.
				if i%5 == 0 {
					time.Sleep(5 * time.Millisecond)
				}
				i++
			}
		}(ea, ePriv)
	}

	// Create a few threads that will be repeatedly sending bitfield
	// requests over TCP. We are going to use short ids that will not
	// correspond to any equipment, so that these requests will continue to
	// produce errors even once the GCA key has been submitted.
	for i := 0; i < 3; i++ {
		go func() {
			// Try until the stop signal is sent.
			i := 0
			for {
				// Try requesting a bitfield.
				_, _, err := gcas.requestEquipmentBitfield(3444555666)
				if err == nil {
					t.Fatal("error expected")
				}

				// Check for the stop signal.
				select {
				case <-stopSignal:
					return
				default:
				}

				// Wait 10 milliseconds between every 5th
				// attempt to minimize cpu spam.
				if i%5 == 0 {
					time.Sleep(5 * time.Millisecond)
				}
				i++
			}
		}()
	}

	// Let everything run for a bit before the test ends. Most of the loops
	// sleep for 10 milliseconds at a time and then do work, so every loop
	// should get plenty of iterations in within 250 milliseconds.
	time.Sleep(250 * time.Millisecond)

	// Send a GCA key to the server. All of the above threads should be
	// able to continue running successfully after that, as they were
	// designed not care whether the GCA key was active or not.
	gcaPrivKey, err := gcas.submitGCAKey(tempPrivKey)
	if err != nil {
		t.Fatal(err)
	}

	// Create a ShortID counter that can be shared across all threads that
	// are authorizing hardware.
	atomicShortID := uint32(0)

	// Create a few threads that will be repeatedly submitting GCA keys
	// with the right signature. Since there's already a key, it should
	// fail.
	for i := 0; i < 3; i++ {
		// Create a goroutine to repeatedly submit the bad key .
		go func(key PrivateKey) {
			// Try until the stop signal is sent.
			i := 0
			for {
				// Try submitting the key.
				_, err := gcas.submitGCAKey(key)
				if err == nil {
					t.Fatal("should not be able to submit a GCA key after it's been set")
				}

				// Check for the stop signal.
				select {
				case <-stopSignal:
					return
				default:
				}

				// Wait 10 milliseconds between every 5th
				// attempt to minimize cpu spam.
				if i%5 == 0 {
					time.Sleep(5 * time.Millisecond)
				}
				i++
			}
		}(tempPrivKey)
	}

	// Create a few threads that will be repeatedly authorizing new
	// equipment.
	for i := 0; i < 3; i++ {
		// Create a goroutine to repeatedly authorize new hardware.
		go func(key PrivateKey) {
			// Try until the stop signal is sent.
			i := 0
			for {
				// Try submitting some new hardware.
				shortID := atomic.AddUint32(&atomicShortID, 1)
				_, _, err := gcas.submitNewHardware(shortID, key)
				if err != nil {
					t.Fatal("should be able to submit new hardware")
				}

				// Check for the stop signal.
				select {
				case <-stopSignal:
					return
				default:
				}

				// Wait 10 milliseconds between every 5th
				// attempt to minimize cpu spam.
				if i%5 == 0 {
					time.Sleep(5 * time.Millisecond)
				}
				i++
			}
		}(gcaPrivKey)
	}

	// Create a few threads that will use the same authorized equipment to
	// submit reports. This is the first place where we will be actually
	// advancing the flow of time.
	for i := 0; i < 3; i++ {
		// Create authorized equipment to be making reports.
		shortID := atomic.AddUint32(&atomicShortID, 1)
		ea, ePriv, err := gcas.submitNewHardware(shortID, gcaPrivKey)
		if err != nil {
			t.Fatal(err)
		}
		go func(ea EquipmentAuthorization, ePriv PrivateKey, threadNum int) {
			// Try until the stop signal is sent.
			i := 0
			resends := 0
			for {
				// Before submitting a new report, advance the
				// time.
				slot := atomic.AddUint32(&manualCurrentTimeslot, 1)

				// Try submitting a new report.
				err := gcas.sendEquipmentReportSpecific(ea, ePriv, slot, 5)
				if err != nil {
					t.Fatal("reports should send correctly")
				}

				// For threads 2 and 3, we use
				// requestEquipmentBitfield to submit missing
				// reports.
				if threadNum == 1 || threadNum == 2 {
					// Get the bitfield that says which
					// reports are missing.
					offset, bitfield, err := gcas.requestEquipmentBitfield(ea.ShortID)
					if err != nil {
						t.Fatal(err)
					}
					for i := 0; i < len(bitfield)*8 && uint32(i)+offset < currentTimeslot(); i++ {
						byteIndex := i/8
						bitIndex := i%8
						mask := byte(1 << bitIndex)
						if bitfield[byteIndex] & mask != 0 {
							continue
						}
						err := gcas.sendEquipmentReportSpecific(ea, ePriv, offset+uint32(i), 5)
						if err != nil {
							t.Fatal(err)
						}
						resends++
					}
				}

				// Check for the stop signal.
				select {
				case <-stopSignal:
					// Log the percentage of requests that
					// seem to have gone through.
					gcas.mu.Lock()
					totalGood := 0
					totalBad := 0
					for i := gcas.equipmentReportsOffset; i < currentTimeslot() && i < gcas.equipmentReportsOffset+4032; i++ {
						if gcas.equipmentReports[ea.ShortID][i-gcas.equipmentReportsOffset].PowerOutput > 1 {
							totalGood++
						} else {
							totalBad++
						}
					}
					gcas.mu.Unlock()

					// There are three threads marching
					// time forward, so we expect a read
					// rate of 1/3rd.
					//
					// NOTE: The target rate of 1/3rd will
					// change if the number of threads that
					// are changing the time also changes.
					t.Logf("equipment %v hit rate: %v :: %v :: %v :: %v", ea.ShortID, float64(totalGood)/float64(totalGood+totalBad), totalGood+totalBad, i, resends)
					return
				default:
				}

				// Wait 10 milliseconds between every 5th
				// attempt to minimize cpu spam.
				if i%5 == 0 {
					time.Sleep(5 * time.Millisecond)
				}
				i++
			}
		}(ea, ePriv, i)
		go func(e EquipmentAuthorization) {
			// Try until the stop signal is sent.
			i := 0
			for {
				// Get the bitfield that says which reports are
				// missing.
				_, _, err := gcas.requestEquipmentBitfield(e.ShortID)
				if err != nil {
					t.Fatal(err)
				}

				// Check for the stop signal.
				select {
				case <-stopSignal:
					return
				default:
				}

				// Wait 10 milliseconds between every 5th
				// attempt to minimize cpu spam.
				if i%5 == 0 {
					time.Sleep(5 * time.Millisecond)
				}
				i++
			}
		}(ea)
	}

	// Run for another 250 milliseconds to let the new loops have some time
	// to generate chaos.
	time.Sleep(250 * time.Millisecond)

	// Stop everything, and then wait 50 more milliseconds for good measure.
	close(stopSignal)
	time.Sleep(50 * time.Millisecond)
}
