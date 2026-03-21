package intel_gpu_top

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/clambin/intel-gpu-exporter/intel-gpu-top/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryBytes_JSON(t *testing.T) {
	type arg struct {
		Total MemoryBytes
	}
	input := arg{Total: 1024}

	b, err := json.Marshal(input)
	require.NoError(t, err)
	assert.Equal(t, `{"Total":"1024"}`, string(b))

	var output arg
	require.NoError(t, json.Unmarshal(b, &output))
	assert.Equal(t, input, output)
}

func TestGPUStats_UnmarshalJSON(t *testing.T) {
	var stats GPUStats
	require.NoError(t, json.Unmarshal([]byte(testutil.SinglePayload), &stats))
	require.Len(t, stats.Engines, 4)
	require.Contains(t, stats.Clients, "4293539623")
	require.Equal(t, "foo", stats.Clients["4293539623"].Name)
	require.Len(t, stats.Clients["4293539623"].EngineClasses, 4)
	require.Len(t, stats.Clients["4293539623"].Memory, 1)
	require.Len(t, stats.Clients["4293539623"].Memory["system"], 5)
}

func TestJSONFormatter(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"with comma's", `[{"foo":"bar"},{"foo":"bar"}]`, `{"foo":"bar"}{"foo":"bar"}`},
		{"without comma's", `[{"foo":"bar"}{"foo":"bar"}]`, `{"foo":"bar"}{"foo":"bar"}`},
		{"white space", ` [ {"foo":"bar"}{"foo":"bar"} ]`, `  {"foo":"bar"}{"foo":"bar"} `},
		{"missing trailing ]", `[{"foo":"bar"},{"foo":"bar"}`, `{"foo":"bar"}{"foo":"bar"}`},
		{"ignore inner array", `[{"key":["foo","bar"]}}]`, `{"key":["foo","bar"]}}`},
		{"nested", `[{"foo":{"bar":0,"baz":1}}]`, `{"foo":{"bar":0,"baz":1}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf, err := io.ReadAll(&jsonFormatter{Source: strings.NewReader(tt.input)})
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(buf))
		})
	}
}

// Now:
// BenchmarkJSONFormatter-10    	   14036	     84990 ns/op	    1632 B/op	       6 allocs/op
func BenchmarkJSONFormatter(b *testing.B) {
	var payload strings.Builder
	payload.WriteString("[")
	for i := range 32 {
		if i > 0 {
			payload.WriteString(",")
		}
		payload.WriteString(testutil.SinglePayload)
	}
	payload.WriteString("]")

	b.ReportAllocs()
	b.ResetTimer()
	var buf [512]byte
	for b.Loop() {
		r := &jsonFormatter{Source: strings.NewReader(payload.String())}

		var err error
		for !errors.Is(err, io.EOF) {
			_, err = r.Read(buf[:])
		}
	}
}

func TestReadGpuStats(t *testing.T) {
	var gpuStats GPUStats
	require.NoError(t, json.Unmarshal([]byte(testutil.SinglePayload), &gpuStats))

	const recordCount = 5
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	buf.WriteString("[")
	for range recordCount {
		_ = encoder.Encode(gpuStats)
		buf.WriteString(",")
	}
	buf.WriteString("]")

	var collectedStats []GPUStats
	for stats, err := range ReadGPUStats(&buf) {
		require.NoError(t, err)
		assert.Equal(t, gpuStats, stats)
		collectedStats = append(collectedStats, stats)
	}
	assert.Len(t, collectedStats, recordCount)
	for _, stat := range collectedStats {
		assert.Equal(t, gpuStats, stat)
	}
}

func TestReadGpuStats_Invalid(t *testing.T) {
	var returnedStats int
	for stats, err := range ReadGPUStats(strings.NewReader("invalid json")) {
		assert.Error(t, err)
		assert.Zero(t, stats)
		returnedStats++
	}
	assert.Equal(t, 1, returnedStats)
}

// Now:
// BenchmarkReadGpuStats-10    	    2622	    397378 ns/op	  126529 B/op	    2134 allocs/op
func BenchmarkReadGpuStats(b *testing.B) {
	var payload strings.Builder
	payload.WriteString("[")
	for i := range 32 {
		if i > 0 {
			payload.WriteString(",")
		}
		payload.WriteString(testutil.SinglePayload)
	}
	payload.WriteString("]")

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		var count int
		for _, err := range ReadGPUStats(strings.NewReader(payload.String())) {
			if err != nil {
				b.Fatal(err)
			}
			count++
		}
		if count != 32 {
			b.Fatalf("expected 32 stats, got %d", count)
		}
	}
}
