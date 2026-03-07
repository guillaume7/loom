package fsm_test

import (
	"testing"

	"github.com/guillaume7/loom/internal/fsm"
	"github.com/stretchr/testify/assert"
)

func TestState_StringValues(t *testing.T) {
	tests := []struct {
		state fsm.State
		want  string
	}{
		{fsm.StateIdle, "IDLE"},
		{fsm.StateScanning, "SCANNING"},
		{fsm.StateComplete, "COMPLETE"},
	}
	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			assert.Equal(t, tt.want, string(tt.state))
		})
	}
}
