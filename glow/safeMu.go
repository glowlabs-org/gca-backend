package glow

// safeMu is an alternative implementation of sync.Mutex which tracks what
// goroutines are holding the mutex and dumps a stack if there's a goroutine
// that was holding a mutex for more than 20 seconds. It can be swapped out
// with a sync.Mutex seamlessly, and will provide useful debugging info if a
// deadlock or other mutex mistake shows up during testing.

import (
	"bytes"
	"fmt"
	"runtime"
	"strconv"
	"sync"
	"time"
)

// finderMutex is a helper to locate deadlocks.
type finderMutex struct {
	lockMap map[int]struct{}
	lmMu    sync.Mutex
	once    sync.Once

	mu sync.Mutex
}

// Discouraged by the go developers, but it's the technique that I know to
// detect when a thread has double locked a mutex.
func getGoroutineID() int {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	// Parse the 4707 out of "goroutine 4707 [running]:"
	field := bytes.Fields(buf[:n])[1]
	id, err := strconv.Atoi(string(field))
	if err != nil {
		panic(fmt.Sprintf("cannot get goroutine id: %v", err))
	}
	return id
}

// Wraps the sync.Mutex Lock() call with some logic that records the goroutine
// in a map, and then checks 20 seconds later if the goroutine is still holding
// the mutex.
//
// TODO: Technically, we should be putting in the map what lock number this is,
// because you could get a false positive if a single goroutine is locking for
// long periods of time and then only briefly unlocking occasionally.
func (fm *finderMutex) Lock() {
	// Create a buffer large enough to hold the stack trace
	buf := make([]byte, 8192)      // 8192 is probably enough to get the full stack trace for one goroutine
	n := runtime.Stack(buf, false) // Set 'false' to get only the current goroutine's stack

	// Get the goroutine ID and check for a double lock.
	id := getGoroutineID()
	fm.lmMu.Lock()
	fm.once.Do(func() {
		fm.lockMap = make(map[int]struct{})
	})
	_, exists := fm.lockMap[id]
	if exists {
		fm.lmMu.Unlock()
		fmt.Println("duplicate goroutine id, double lock")
		fmt.Printf("%s\n", buf[:n])
	}
	fm.lockMap[id] = struct{}{}
	fm.lmMu.Unlock()

	// Set up a thread to make sure the gorountine has released the mutex
	// after 20 seconds.
	//
	// TODO: This isn't strictly reliable, because the same thread may be
	// locking and unlocking the mutex repeatedly, which would cause a
	// false positive. What we actually need to do is record which lock
	// count this is, and the goroutine needs to make sure that if the
	// mutex is still locked after 20 seconds, it's a different lock
	// attempt than the attempt associated with this thread.
	go func() {
		time.Sleep(time.Second * 20)
		fm.lmMu.Lock()
		_, exists := fm.lockMap[id]
		if exists {
			fmt.Println("we didn't let go of this lock")
			fmt.Printf("%s\n", buf[:n])
		}
		fm.lmMu.Unlock()
	}()
	fm.mu.Lock()
}

// Unlock will delete this goroutine from the lock map so the sleeping
// goroutine will see that the mutex was released correctly.
func (fm *finderMutex) Unlock() {
	fm.lmMu.Lock()
	id := getGoroutineID()
	_, exists := fm.lockMap[id]
	if !exists {
		fm.lmMu.Unlock()
		panic("bad")
	}
	delete(fm.lockMap, id)
	fm.lmMu.Unlock()
	fm.mu.Unlock()
}
