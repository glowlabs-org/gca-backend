package glow

// safeMu is an alternative implementation of sync.Mutex which tracks what
// goroutines are holding the mutex and dumps a stack if there's a goroutine
// that was holding a mutex for more than 20 seconds. It can be swapped out
// with a sync.Mutex seamlessly, and will provide useful debugging info if a
// deadlock or other mutex mistake shows up during testing.

// NOTE: The SafeMu object may be viewed as violating the Glow concurrency
// convention that states "mutexes are not allowed to stack." This is because
// the lockMap mutex is grabbed while the SafeMu mutex is being held during the
// Unlock() call.
//
// SafeMu is able to do this safely because it is easily apparent that the
// lockMap mutex is always only ever held within the context of a single call,
// and there's no codepath in this file that allows the lockMap mutex to be
// held while calling out to some external function.
//
// The lockMap mutex can therefore be considered an underclass mutex, safely
// excluding it from the mutex stacking convetion.
//
// Underclass mutexes are an advanced idea, and it's generally ill advised to
// use them because of the risk of incorrect use.

import (
	"bytes"
	"fmt"
	"runtime"
	"strconv"
	"sync"
	"time"
)

// lockMap is a psecial
type lockMap struct {
	counts map[int]int
	locks  map[int]struct{}
	mu     sync.Mutex
}

// SafeMu is an object that wraps sync.Mutex to help locate deadlocks.
type SafeMu struct {
	lm   lockMap
	mu   sync.Mutex
	once sync.Once
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
func (sm *SafeMu) Lock() {
	// Create a buffer large enough to hold the stack trace
	buf := make([]byte, 8192)      // 8192 is probably enough to get the full stack trace for one goroutine
	n := runtime.Stack(buf, false) // Set 'false' to get only the current goroutine's stack

	// Get the goroutine ID and check for a double lock.
	id := getGoroutineID()
	sm.lm.mu.Lock()
	sm.once.Do(func() {
		sm.lm.locks = make(map[int]struct{})
		sm.lm.counts = make(map[int]int)
	})
	_, exists := sm.lm.locks[id]
	if exists {
		sm.lm.mu.Unlock()
		fmt.Println("A goroutine appears to have double-locked a mutex:")
		fmt.Printf("%s\n", buf[:n])
	}
	sm.lm.locks[id] = struct{}{}
	count := 1 + sm.lm.counts[id]
	sm.lm.counts[id] = count
	sm.lm.mu.Unlock()

	// Set up a thread to make sure the gorountine has released the mutex.
	// Print the stack if the mutex has been held for too long, and keep
	// printing the stack every few seconds for as long as the mutex is not
	// released.
	var detectDeadlock func(time.Duration)
	detectDeadlock = func(i time.Duration) {
		sleepSeconds := time.Duration(20) // Modify as needed while debugging
		time.Sleep(time.Second * sleepSeconds)
		sm.lm.mu.Lock()
		_, exists := sm.lm.locks[id]
		if exists && sm.lm.counts[id] == count {
			fmt.Printf("A lock has been held for more than %v seconds:\n", i*sleepSeconds)
			fmt.Printf("%s\n", buf[:n])
			go detectDeadlock(i + 1)
		}
		sm.lm.mu.Unlock()
	}
	go detectDeadlock(0)

	sm.mu.Lock()
}

// Unlock will delete this goroutine from the lock map so the sleeping
// goroutine will see that the mutex was released correctly.
func (sm *SafeMu) Unlock() {
	sm.lm.mu.Lock()
	id := getGoroutineID()
	_, exists := sm.lm.locks[id]
	if !exists {
		// Create a buffer large enough to hold the stack trace
		buf := make([]byte, 8192)      // 8192 is probably enough to get the full stack trace for one goroutine
		n := runtime.Stack(buf, false) // Set 'false' to get only the current goroutine's stack
		fmt.Println("Unlock called on mutex from goroutine that is not currently holding the lock")
		fmt.Printf("%s\n", buf[:n])
	} else {
		delete(sm.lm.locks, id)
	}
	sm.lm.mu.Unlock()
	sm.mu.Unlock()
}
