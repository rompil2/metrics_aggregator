package repository

import (
	"fmt"
	"sync"
)

type MemStorage struct {
	storage sync.Map
}

// Returns a pointer to the MemStorage
func NewMemStorage() (*MemStorage, error) {
	mem := MemStorage{
		sync.Map{},
	}
	return &mem, nil
}

// Sets value in the Mem Storage
func (mem *MemStorage) SetMetrics(ID string, value any) error {
	mem.storage.Store(ID, value)
	return nil
}

// Gets a value by ID and return an error if ID is unknown
func (mem *MemStorage) GetMetrics(ID string) (any, error) {
	val, ok := mem.storage.Load(ID)
	if !ok {
		return nil, fmt.Errorf("requested value for %s does not exist\n", ID)
	}
	return val, nil
}

// Gets all values from the Mem Storage
func (mem *MemStorage) AllMetrics() ([]any, error) {

	var result []any
	mem.storage.Range(func(key, value any) bool {
		result = append(result, value)
		return true
	})
	return result, nil

}
