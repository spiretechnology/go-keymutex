package keymutex

import "sync"

// KeyMutex is a mutex that can be locked and unlocked on arbitrary keys.
type KeyMutex[T comparable] struct {
	mut       sync.Mutex
	m         map[T]*sync.Mutex
	refCounts map[T]int
}

// Lock locks the mutex for the given key.
func (km *KeyMutex[T]) Lock(key T) {
	km.lockWithWaiting(key, nil)
}

// Unlock unlocks the mutex for the given key.
func (km *KeyMutex[T]) Unlock(key T) {
	// Acquire the map lock
	km.mut.Lock()
	defer km.mut.Unlock()

	// Ensure Unlock is not called more times than Lock
	if km.refCounts[key] <= 0 {
		return
	}

	// Get the mutex for the key
	mut := km.m[key]

	// Decrement the counter for the key
	km.refCounts[key]--

	// If the counter is zero, delete the mutex
	if km.refCounts[key] == 0 {
		delete(km.m, key)
		delete(km.refCounts, key)
	}

	// Unlock the mutex
	mut.Unlock()
}

func (km *KeyMutex[T]) lockWithWaiting(key T, chanCallback chan<- struct{}) {
	// Acquire the map lock
	km.mut.Lock()

	// Ensure the map exists
	if km.m == nil {
		km.m = map[T]*sync.Mutex{}
	}
	if km.refCounts == nil {
		km.refCounts = map[T]int{}
	}

	// Get the mutex for the key. Create it if it doesn't exist
	mut, ok := km.m[key]
	if !ok {
		mut = &sync.Mutex{}
		km.m[key] = mut
	}

	// Increment the counter for the key
	km.refCounts[key]++

	// Lock the mutex
	if chanCallback != nil {
		chanCallback <- struct{}{}
	}
	km.mut.Unlock()
	mut.Lock()
}
