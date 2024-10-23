package intel_gpu_top

import (
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestV118ArrayRemover(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{
			name:  "v1.17",
			input: realPayload + realPayload + realPayload,
			want:  3,
		},
		{
			name:  "v1.18",
			input: "[" + realPayload + realPayload + "]\n\n\n",
			want:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &V118ArrayRemover{Reader: strings.NewReader(tt.input)}
			var got int
			for _, err := range ReadGPUStats(r) {
				assert.NoError(t, err)
				got++
			}
			assert.Equal(t, tt.want, got)
		})
	}
}
