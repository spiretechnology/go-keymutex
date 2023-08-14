package keymutex

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKeyMutex(t *testing.T) {
	var km KeyMutex[int]
	var wg sync.WaitGroup

	var sequence1, sequence2 []string
	key1 := 1
	key2 := 2

	km.Lock(1)

	// In the background, queue a sequence of events
	wg.Add(2)
	go func() {
		defer wg.Done()
		km.Lock(key1)
		require.Equal(t, 1, km.refCounts[key1], "refCounts[key1] should be 1")
		defer km.Unlock(key1)
		go func() {
			defer wg.Done()
			km.Lock(key1)
			defer km.Unlock(key1)
			sequence1 = append(sequence1, "C")
		}()
		sequence1 = append(sequence1, "B")
		km.Unlock(key1)
	}()

	// This should not deadlock, even though key1 is already locked
	km.Lock(key2)
	require.Equal(t, 1, km.refCounts[key2], "refCounts[key2] should be 1")
	sequence2 = append(sequence2, "A")
	km.Unlock(key2)
	key2RefCount, key2RefCountOk := km.refCounts[key2]
	require.Equal(t, 0, key2RefCount, "refCounts[key2] should be 0")
	require.Equal(t, false, key2RefCountOk, "refCounts[key2] should not exist")

	// Add to the sequence and unlock the key, allowing the goroutines to continue
	sequence1 = append(sequence1, "A")
	km.Unlock(key1)

	// Wait for the goroutines to finish
	wg.Wait()

	require.Equal(t, []string{"A", "B", "C"}, sequence1)
	require.Equal(t, []string{"A"}, sequence2)
	require.Equal(t, 0, km.refCounts[key1], "refCounts[key1] should be 0")
	require.Equal(t, 0, km.refCounts[key2], "refCounts[key2] should be 0")
}

func TestKeyMutexLocking(t *testing.T) {
	var km KeyMutex[int]
	var wgAcquiringLock sync.WaitGroup
	var wgAllLocksReleased sync.WaitGroup
	iterCount := 5
	var grantedCount int

	km.Lock(1)

	chanUnsuspend := make(chan struct{})

	// Queue up a bunch of goroutines waiting to acquire the same lock
	for i := 0; i < iterCount; i++ {
		wgAcquiringLock.Add(1)
		wgAllLocksReleased.Add(1)
		go func() {
			defer wgAllLocksReleased.Done()
			defer km.Unlock(1)
			chanWaiting := make(chan struct{})
			go func() {
				<-chanWaiting
				wgAcquiringLock.Done()
			}()
			km.lockWithWaiting(1, chanWaiting)
			<-chanUnsuspend
			grantedCount++
		}()
	}

	// Because we acquired the first lock, the grantedCount should still be zero here
	require.Equal(t, 0, grantedCount, "grantedCount should be 0")

	// Wait for all goroutines to be waiting to acquire the lock
	wgAcquiringLock.Wait()
	require.Equal(t, iterCount+1, km.refCounts[1], "refCounts[1] should be %d", iterCount+1)

	// Allow all locks to be acquired sequentially
	km.Unlock(1)
	close(chanUnsuspend)

	// Acquire one more lock, which should wait until all the other locks are released
	wgAllLocksReleased.Wait()
	require.Equal(t, 0, km.refCounts[1], "refCounts[1] should be 0")
	require.Equal(t, iterCount, grantedCount, "grantedCount should be %d", iterCount)
}
