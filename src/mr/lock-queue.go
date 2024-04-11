package mr

import "sync"

// SliceQueue is an unbounded queue which uses a slice as underlying.
type SliceQueue struct {
	data [][]string
	mu   sync.Mutex
}

// NewSliceQueue returns an empty queue.
// You can give a initial capacity.
func NewSliceQueue(n int) (q *SliceQueue) {
	return &SliceQueue{data: make([][]string, 0, n)}
}

// Enqueue puts the given value v at the tail of the queue.
func (q *SliceQueue) Enqueue(v []string) {
	q.mu.Lock()
	q.data = append(q.data, v)
	q.mu.Unlock()
}

// Dequeue removes and returns the value at the head of the queue.
// It returns nil if the queue is empty.
func (q *SliceQueue) Dequeue() []string {
	q.mu.Lock()
	if len(q.data) == 0 {
		q.mu.Unlock()
		return make([]string, 0)
	}
	v := q.data[0]
	q.data = q.data[1:]
	q.mu.Unlock()
	return v
}
