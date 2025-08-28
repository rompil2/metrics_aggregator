package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewMemStorage(t *testing.T) {
	_, err := NewMemStorage()
	if err != nil {
		t.Fatal(
			"Can't create new MemStorage",
			err,
		)
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
			mm, err := NewMemStorage()
			if err != nil {
				t.Fatal("Can't create new MemStorage", err)
			}
			err = mm.SetMetrics(id, tc.value)
			if tc.wanrErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			got, err := mm.GetMetrics(id)
			assert.NoError(t, err)
			if err == nil {
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

func TestMemStorage_GetValue(t *testing.T) {
	tt := []struct {
		name    string
		value   any
		want    any
		wanrErr bool
	}{{
		"Get value", int(1), int(1), false,
	}}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			id := "test"
			mm, err := NewMemStorage()
			if err != nil {
				t.Fatal("Can't create new MemStorage", err)
			}
			err = mm.SetMetrics(id, tc.value)
			if err != nil {
				t.Fatal("Can't set value", err)
			}
			got, err := mm.GetMetrics(id)
			if tc.wanrErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			if err == nil {
				if got != nil {
					assert.Equal(t, tc.want, got)
				}
			}
		})
	}
}

func TestMemStorage_AllValues(t *testing.T) {
	t.Run("Get all values", func(t *testing.T) {

		mm, err := NewMemStorage()
		if err != nil {
			t.Fatal("Can't create new MemStorage", err)
		}
		id := "test"
		err = mm.SetMetrics(id, int(1))
		if err != nil {
			t.Fatal("Can't set value", err)
		}
		id = "test1"
		err = mm.SetMetrics(id, int(2))
		if err != nil {
			t.Fatal("Can't set value", err)
		}
		got, err := mm.AllMetrics()
		assert.NoError(t, err)
		if err == nil {
			if got != nil {
				assert.Equal(t, 2, len(got))
			}
		}
	})
}
