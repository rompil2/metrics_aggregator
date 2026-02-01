package pool

import (
	"sync"
)

// Resetter describes types that have a Reset method.
type Resetter interface {
	Reset()
}

// Pool wraps sync.Pool to work with types that implement Resetter.
type Pool[T Resetter] struct {
	pool *sync.Pool
}

// New creates and returns a new Pool for type T.
func New[T Resetter](newFunc func() T) *Pool[T] {
	return &Pool[T]{
		pool: &sync.Pool{
			New: func() any {
				return newFunc()
			},
		},
	}
}

// Get returns an object of type T from the pool.
func (p *Pool[T]) Get() T {
	obj := p.pool.Get().(T)
	return obj
}

// Put places an object of type T back into the pool after calling Reset().
func (p *Pool[T]) Put(obj T) {
	obj.Reset()
	p.pool.Put(obj)
}
