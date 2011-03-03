package queue

import (
	"sync"
)

type Queue struct {
	items map[int64]chan bool
	mu    *sync.Mutex
}

func (q *Queue) Add(id int64, ch chan bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items[id] = ch
}

func (q *Queue) Remove(id int64) {
	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.items, id)
}

func (q *Queue) Flush() {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, v := range q.items {
		v <- true
	}
	q.items = map[int64]chan bool{}
}

func (q *Queue) List() []int64 {
	q.mu.Lock()
	defer q.mu.Unlock()
	l := make([]int64, len(q.items))
	idx := 0
	for k, _ := range q.items {
		l[idx] = k
		idx++
	}
	return l
}

func New() *Queue {
	q := Queue{}
	q.items = map[int64]chan bool{}
	q.mu = &sync.Mutex{}
	return &q
}
