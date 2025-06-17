package intel_gpu_top

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/clambin/intel-gpu-exporter/pkg/intel-gpu-top/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJsonTracker(t *testing.T) {
	tests := []struct {
		input    string
		wantBody string
		wantOk   bool
	}{
		{`{ "foo": "bar" `, ``, false},
		{`}`, `{ "foo": "bar" }`, true},
		{`{ "foo": "bar" }`, `{ "foo": "bar" }`, true},
		{`{ "foo": "bar`, ``, false},
		{`" }`, `{ "foo": "bar" }`, true},
		{`{ "foo": "\"bar\"" }`, `{ "foo": "\"bar\"" }`, true},
	}

	var tracker jsonTracker
	for _, tt := range tests {
		for _, ch := range tt.input {
			tracker.Process(byte(ch))
		}
		r, ok := tracker.HasCompleteObject()
		require.Equal(t, tt.wantOk, ok)
		if ok {
			require.NotNil(t, r)
			var buf bytes.Buffer
			_, err := r.WriteTo(&buf)
			require.NoError(t, err)
			assert.Equal(t, tt.wantBody, buf.String())
		}
	}
}

// Now:
// BenchmarkV118toV117-10    	   28198	     41222 ns/op	    6064 B/op	      10 allocs/op
func BenchmarkV118toV117(b *testing.B) {
	// generate input outside the benchmark
	var payload bytes.Buffer
	r := testutil.FakeServer(context.Background(), []byte(testutil.SinglePayload), 10, true, true, 0*time.Millisecond)
	if _, err := payload.ReadFrom(r); err != nil {
		b.Fatal(err)
	}
	buf := make([]byte, 2048)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		r = &V118toV117{Source: bytes.NewReader(payload.Bytes())}
		var err error
		for !errors.Is(err, io.EOF) {
			clear(buf)
			_, err = r.Read(buf)
		}
	}
}

var (
	gpuStatV117 = GPUStats{
		Engines: map[string]EngineStats{
			"Blitter/0":      {Unit: "%", Busy: 1, Sema: 0, Wait: 0},
			"Render/3D/0":    {Unit: "%", Busy: 2, Sema: 0, Wait: 0},
			"Video/0":        {Unit: "%", Busy: 3, Sema: 0, Wait: 0},
			"VideoEnhance/0": {Unit: "%", Busy: 4, Sema: 0, Wait: 0}},
		Period: struct {
			Unit     string  "json:\"unit\""
			Duration float64 "json:\"duration\""
		}{Unit: "ms", Duration: 0.295595},
		Interrupts: struct {
			Unit  string  "json:\"unit\""
			Count float64 "json:\"count\""
		}{Unit: "irq/s", Count: 0},
		Rc6: struct {
			Unit  string  "json:\"unit\""
			Value float64 "json:\"value\""
		}{Unit: "%", Value: 0},
		Frequency: struct {
			Unit      string  "json:\"unit\""
			Requested float64 "json:\"requested\""
			Actual    float64 "json:\"actual\""
		}{Unit: "MHz", Requested: 0, Actual: 0}, Power: struct {
			Unit    string  "json:\"unit\""
			GPU     float64 "json:\"GPU\""
			Package float64 "json:\"Package\""
		}{Unit: "W", GPU: 0, Package: 0}, ImcBandwidth: struct {
			Unit   string  "json:\"unit\""
			Reads  float64 "json:\"reads\""
			Writes float64 "json:\"writes\""
		}{Unit: "MiB/s", Reads: 1084.032444, Writes: 0.412965},
		Clients: map[string]ClientStats{},
	}

	gpuStatV118 = GPUStats{
		Engines: map[string]EngineStats{
			"Blitter":      {Unit: "%", Busy: 2, Sema: 0, Wait: 0},
			"Render/3D":    {Unit: "%", Busy: 1, Sema: 0, Wait: 0},
			"Video":        {Unit: "%", Busy: 3, Sema: 0, Wait: 0},
			"VideoEnhance": {Unit: "%", Busy: 4, Sema: 0, Wait: 0}},
		Period: struct {
			Unit     string  "json:\"unit\""
			Duration float64 "json:\"duration\""
		}{Unit: "ms", Duration: 25.262162},
		Interrupts: struct {
			Unit  string  "json:\"unit\""
			Count float64 "json:\"count\""
		}{Unit: "irq/s", Count: 0},
		Rc6: struct {
			Unit  string  "json:\"unit\""
			Value float64 "json:\"value\""
		}{Unit: "%", Value: 100},
		Frequency: struct {
			Unit      string  "json:\"unit\""
			Requested float64 "json:\"requested\""
			Actual    float64 "json:\"actual\""
		}{Unit: "MHz", Requested: 0, Actual: 0},
		Power: struct {
			Unit    string  "json:\"unit\""
			GPU     float64 "json:\"GPU\""
			Package float64 "json:\"Package\""
		}{Unit: "W", GPU: 0, Package: 36.407762},
		ImcBandwidth: struct {
			Unit   string  "json:\"unit\""
			Reads  float64 "json:\"reads\""
			Writes float64 "json:\"writes\""
		}{Unit: "MiB/s", Reads: 2170.189132, Writes: 651.498156},
		Clients: map[string]ClientStats{
			"4294879639": {
				Name: "foo",
				Pid:  "87657",
				EngineClasses: map[string]struct {
					Busy string `json:"busy"`
					Unit string `json:"unit"`
				}{
					"Blitter":      {Busy: "0.000000", Unit: "%"},
					"Render/3D":    {Busy: "75.449180", Unit: "%"},
					"Video":        {Busy: "20.849516", Unit: "%"},
					"VideoEnhance": {Busy: "0.000000", Unit: "%"},
				},
			},
		},
	}
)

func TestReadGpuStats(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		v18      bool
		want     []GPUStats
	}{
		{
			name:     "v1.17",
			filename: "igt-v1.17.txt",
			want:     []GPUStats{gpuStatV117, gpuStatV117},
		},
		{
			name:     "v1.18",
			filename: "igt-v1.18.txt",
			v18:      true,
			want:     []GPUStats{gpuStatV118, gpuStatV118},
		},
		{
			name:     "v1.18 with comma's",
			filename: "igt-v1.18-comma.txt",
			v18:      true,
			want:     []GPUStats{gpuStatV118, gpuStatV118},
		},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			f, err := os.Open(filepath.Join("testdata", tt.filename))
			require.NoError(t, err)
			defer func() { _ = f.Close() }()
			var r io.Reader
			r = f
			if tt.v18 {
				r = &V118toV117{Source: f}
			}

			var collectedStats []GPUStats
			for stats, err := range ReadGPUStats(r) {
				require.NoError(t, err)
				collectedStats = append(collectedStats, stats)
			}

			assert.Equal(t, tt.want, collectedStats)
		})
	}
}
