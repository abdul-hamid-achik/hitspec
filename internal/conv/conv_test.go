package conv

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToFloat64(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want float64
		ok   bool
	}{
		{"float64", 3.14, 3.14, true},
		{"float32", float32(2.5), 2.5, true},
		{"int", 42, 42.0, true},
		{"int64", int64(100), 100.0, true},
		{"int32", int32(10), 10.0, true},
		{"string numeric", "3.14", 3.14, true},
		{"string non-numeric", "abc", 0, false},
		{"bool", true, 0, false},
		{"nil", nil, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ToFloat64(tt.in)
			assert.Equal(t, tt.ok, ok)
			if ok {
				assert.InDelta(t, tt.want, got, 0.001)
			}
		})
	}
}
