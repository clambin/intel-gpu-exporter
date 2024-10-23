package intel_gpu_top

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestAggregator(t *testing.T) {
	var a Aggregator
	assert.NoError(t, a.Process(statWriter(2, []byte(statEntry), time.Millisecond)))

	// a.Read works asynchronously. Wait for all data to be read.
	assert.Eventually(t, func() bool { return len(a.EngineStats()) == 4 }, time.Second, time.Millisecond)

	engineStats := a.EngineStats()
	require.Len(t, engineStats, 4)
	for i, engineName := range []string{"Render/3D", "Blitter", "Video", "VideoEnhance"} {
		assert.Contains(t, engineStats, engineName)
		assert.Equal(t, float64(i+1), engineStats[engineName].Busy)
		assert.Equal(t, "%", engineStats[engineName].Unit)
	}

	assert.Equal(t, 1.0, a.ClientStats())
}

func Test_medianFunc(t *testing.T) {
	t.Run("float64", func(t *testing.T) {
		values := make([]float64, 5)
		for i := range 5 {
			values[i] = float64(i)
		}
		assert.Equal(t, float64(2), medianFunc(values, func(f float64) float64 { return f }))
		values = make([]float64, 6)
		for i := range 6 {
			values[i] = float64(i)
		}
		assert.Equal(t, 2.5, medianFunc(values, func(f float64) float64 { return f }))

		assert.Zero(t, medianFunc(nil, func(f float64) float64 { return f }))
	})
}
