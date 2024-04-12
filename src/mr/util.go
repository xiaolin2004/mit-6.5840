package mr

import "sync"

func splitSlice(slice []KeyValue, n int) [][]KeyValue {
	var result [][]KeyValue
	size := (len(slice) / n) + 1
	for i := 0; i < n; i++ {
		start := i * size
		end := start + size
		if i == n-1 {
			end = len(slice)
		}
		result = append(result, slice[start:end])
	}
	return result
}

// for sorting by key.
type ByKey []KeyValue

// for sorting by key.
func (a ByKey) Len() int           { return len(a) }
func (a ByKey) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByKey) Less(i, j int) bool { return a[i].Key < a[j].Key }

type LockMIndex struct {
	index int
	mu    sync.Mutex
}

func (index *LockMIndex) Increment() {
	index.mu.Lock()
	index.index++
	index.mu.Unlock()
}

func (index *LockMIndex) Read() int {
	index.mu.Lock()
	defer index.mu.Unlock()
	return index.index
}
