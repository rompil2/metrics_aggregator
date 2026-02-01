package pool

import (
	"sync"
)

// Resetter describes types that have a Reset method.
type Resetter interface {
	Reset()
}

// Pool wraps sync.Pool to work with types that implement Resetter.
// It ensures that objects returned to the pool have their state reset before reuse.
type Pool[T Resetter] struct {
	pool *sync.Pool
}

// New creates and returns a new Pool for type T.
// The newFunc parameter specifies how to create a new instance of T when the pool is empty.
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
// If the pool is empty, a new object is created using the constructor provided to New.
func (p *Pool[T]) Get() T {
	obj := p.pool.Get().(T)
	return obj
}

// Put places an object of type T back into the pool after calling Reset().
// This ensures the object's state is cleared before being reused.
func (p *Pool[T]) Put(obj T) {
	obj.Reset()
	p.pool.Put(obj)
}
