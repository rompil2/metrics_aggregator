package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewMemStorage(t *testing.T) {
	got := NewMemStorage()
	if got == nil {
		t.Fatal("MemStorage is Nil")
	}
}

func TestMemStorage_SetValue(t *testing.T) {
	tt := []struct {
		name    string
		value   any
		want    any
		wanrErr bool
	}{{
		"Set value", int(1), int(1), false,
	}}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			id := "test"
			mm := NewMemStorage()
			err := mm.SetValue(id, tc.value)
			if tc.wanrErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			got, err := mm.GetValue(id)
			assert.NoError(t, err)
			if err == nil {
				assert.Equal(t, tc.want, got)
			}
		})
	}
}
