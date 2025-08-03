package repository

import (
	"fmt"
	"sync"
)

type MemStorage struct {
	storage sync.Map
}

// Returns a pointer to the MemStorage
func NewMemStorage() *MemStorage {
	mem := MemStorage{
		sync.Map{},
	}
	return &mem
}

// Sets value in the Mem Storage
func (mem *MemStorage) SetValue(ID string, value any) error {
	mem.storage.Store(ID, value)
	return nil
}

// Gets a value by ID and return an error if ID is unknown
func (mem *MemStorage) GetValue(ID string) (any, error) {
	val, ok := mem.storage.Load(ID)
	if !ok {
		return nil, fmt.Errorf("requested value for %s does not exist", ID)
	}
	return val, nil
}
