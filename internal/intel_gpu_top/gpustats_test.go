package intel_gpu_top

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"strings"
	"testing"
	"time"
)

const statEntry = `
{
	"period":        { "duration": 21.473224, "unit": "ms" },
	"frequency":     { "requested": 0.000000, "actual": 0.000000, "unit": "MHz" },
	"interrupts":    { "count": 1257.379889, "unit": "irq/s" },
	"rc6":           { "value": 100.000000, "unit": "%" },
	"power":         { "GPU": 0.000000, "Package": 35.276832, "unit": "W" },
	"imc-bandwidth": { "reads": 1332.134597, "writes": 129.066989, "unit": "MiB/s" },
	"engines":    {
		"Render/3D":    { "busy": 1.000000, "sema": 0.000000, "wait": 0.000000, "unit": "%" },
		"Blitter":      { "busy": 2.000000, "sema": 0.000000, "wait": 0.000000, "unit": "%" }, 
        "Video":        { "busy": 3.000000, "sema": 0.000000, "wait": 0.000000, "unit": "%" },
		"VideoEnhance": { "busy": 4.000000, "sema": 0.000000, "wait": 0.000000, "unit": "%" }
	},
	"clients": {
        "1": {}
    }
}`

const realPayload = `
{
	"period": {
		"duration": 19.265988,
		"unit": "ms"
	},
	"frequency": {
		"requested": 0.000000,
		"actual": 0.000000,
		"unit": "MHz"
	},
	"interrupts": {
		"count": 0.000000,
		"unit": "irq/s"
	},
	"rc6": {
		"value": 0.000000,
		"unit": "%"
	},
	"power": {
		"GPU": 0.000000,
		"Package": 16.597290,
		"unit": "W"
	},
	"imc-bandwidth": {
		"reads": 1458.945797,
		"writes": 198.603567,
		"unit": "MiB/s"
	},
	"engines": {
		"Render/3D": {
			"busy": 0.000000,
			"sema": 0.000000,
			"wait": 0.000000,
			"unit": "%"
		},
		"Blitter": {
			"busy": 0.000000,
			"sema": 0.000000,
			"wait": 0.000000,
			"unit": "%"
		},
		"Video": {
			"busy": 3.000000,
			"sema": 0.000000,
			"wait": 0.000000,
			"unit": "%"
		},
		"VideoEnhance": {
			"busy": 0.000000,
			"sema": 0.000000,
			"wait": 0.000000,
			"unit": "%"
		}
	},
	"clients": {
		"4293539623": {
			"name": "Plex Transcoder",
			"pid": "1427673",
			"engine-classes": {
				"Render/3D": {
					"busy": "0.000000",
					"unit": "%"
				},
				"Blitter": {
					"busy": "0.000000",
					"unit": "%"
				},
				"Video": {
					"busy": "0.000000",
					"unit": "%"
				},
				"VideoEnhance": {
					"busy": "0.000000",
					"unit": "%"
				}
			}
		}
	}
}`

func TestReadGPUStats_Layouts(t *testing.T) {
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
			name:  "v1.18 (no commas)",
			input: "[\n" + realPayload + "\n" + realPayload + "\n" + realPayload + "\n]\n",
			want:  3,
		},
		/*
			{
				// TODO
				name:  "v1.18 (commas)",
				input: "[\n" + realPayload + ",\n" + realPayload + ",\n" + realPayload + "\n]\n",
				want:  3,
			},
		*/
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &V118ArrayRemover{Reader: strings.NewReader(tt.input)}
			var got int
			for _, err := range ReadGPUStats(r) {
				require.NoError(t, err)
				got++
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func statWriter(count int, payload []byte, interval time.Duration) io.Reader {
	r, w := io.Pipe()
	go func() {
		_, _ = w.Write([]byte("[ \n\n"))
		for range count {
			_, _ = w.Write(payload)
			time.Sleep(interval)
		}
		_, _ = w.Write([]byte("\n]\n"))
		_ = w.Close()
	}()
	return r
}

func BenchmarkReadGPUStats(b *testing.B) {
	for range b.N {
		r := statWriter(10, []byte(statEntry), time.Millisecond)
		var a Aggregator
		if err := a.Process(r); err != nil {
			b.Fatal(err)
		}
	}
}
