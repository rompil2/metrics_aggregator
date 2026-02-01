package pool

import (
	"testing"
)

// MockResetter implements Resetter interface
type MockResetter struct {
	Value       int
	ResetCalled bool
}

func (m *MockResetter) Reset() {
	m.Value = 0
	m.ResetCalled = true
}

func TestPool_New(t *testing.T) {
	p := New(func() *MockResetter {
		return &MockResetter{Value: 100}
	})

	if p == nil {
		t.Fatal("Expected pool to be created, got nil")
	}
}

func TestPool_Get(t *testing.T) {
	p := New(func() *MockResetter {
		return &MockResetter{Value: 100}
	})

	obj := p.Get()

	if obj.Value != 100 {
		t.Errorf("Expected Value=100, got %d", obj.Value)
	}
	if obj.ResetCalled {
		t.Error("Reset should not be called on Get")
	}
}

func TestPool_Put(t *testing.T) {
	p := New(func() *MockResetter {
		return &MockResetter{Value: 100}
	})

	obj := p.Get()
	obj.Value = 200

	p.Put(obj)

	if obj.Value != 0 {
		t.Errorf("Expected Value=0 after Put (Reset), got %d", obj.Value)
	}
	if !obj.ResetCalled {
		t.Error("Expected Reset to be called on Put")
	}
}

func TestPool_GetAfterPut(t *testing.T) {
	p := New(func() *MockResetter {
		return &MockResetter{Value: 100}
	})

	obj1 := p.Get()
	obj1.Value = 500
	p.Put(obj1)

	obj2 := p.Get()

	// obj2 может быть тем же объектом, что и obj1 (повторное использование)
	if obj2.Value != 0 {
		t.Errorf("Expected reused object to have Value=0 after Reset, got %d", obj2.Value)
	}
}
