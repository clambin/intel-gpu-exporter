package intel_gpu_top

import (
	"bytes"
	"context"
	"errors"
	"github.com/clambin/intel-gpu-exporter/pkg/intel-gpu-top/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"testing"
	"time"
)

func TestReadGPUStats(t *testing.T) {
	tests := []struct {
		name    string
		array   bool
		commas  bool
		convert bool
		send    int
		receive int
		wantErr assert.ErrorAssertionFunc
	}{
		{"v1.17", false, false, true, 5, 5, assert.NoError},
		{"v1.18a", true, true, true, 5, 5, assert.NoError},
		{"v1.18b", true, false, true, 5, 5, assert.NoError},
		{"fail", true, true, false, 1, 0, assert.Error},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)

			r := testutil.FakeServer(ctx, []byte(testutil.SinglePayload), tt.send, tt.array, tt.commas, 50*time.Millisecond)

			if tt.convert {
				r = &V118toV117{Source: r}
			}
			var got int
			var err error
			for _, err = range ReadGPUStats(r) {
				if err != nil {
					break
				}
				got++
			}
			tt.wantErr(t, err)
			assert.Equal(t, tt.receive, got)
		})
	}
}

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

// Before:
// BenchmarkV118toV117-16        3372247         354.8 ns/op          64 B/op          2 allocs/op
//
// Now:
// BenchmarkV118toV117-16    	   17662	     67609 ns/op	    6064 B/op	      10 allocs/op
//
// Slower, but more robust.
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
