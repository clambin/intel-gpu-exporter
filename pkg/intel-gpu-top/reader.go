package intel_gpu_top

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
)

// GPUStats contains GPU utilization, as presented by intel-gpu-top
type GPUStats struct {
	Engines map[string]EngineStats `json:"engines"`
	Clients map[string]ClientStats `json:"clients"`
	Period  struct {
		Unit     string  `json:"unit"`
		Duration float64 `json:"duration"`
	} `json:"period"`
	Interrupts struct {
		Unit  string  `json:"unit"`
		Count float64 `json:"count"`
	} `json:"interrupts"`
	Rc6 struct {
		Unit  string  `json:"unit"`
		Value float64 `json:"value"`
	} `json:"rc6"`
	Frequency struct {
		Unit      string  `json:"unit"`
		Requested float64 `json:"requested"`
		Actual    float64 `json:"actual"`
	} `json:"frequency"`
	Power struct {
		Unit    string  `json:"unit"`
		GPU     float64 `json:"GPU"`
		Package float64 `json:"Package"`
	} `json:"power"`
	ImcBandwidth struct {
		Unit   string  `json:"unit"`
		Reads  float64 `json:"reads"`
		Writes float64 `json:"writes"`
	} `json:"imc-bandwidth"`
}

// EngineStats contains the utilization of one GPU engine.
type EngineStats struct {
	Unit string  `json:"unit"`
	Busy float64 `json:"busy"`
	Sema float64 `json:"sema"`
	Wait float64 `json:"wait"`
}

// ClientStats contains statistics for one client, currently using the GPU.
type ClientStats struct {
	EngineClasses map[string]struct {
		Busy string `json:"busy"`
		Unit string `json:"unit"`
	} `json:"engine-classes"`
	Name string `json:"name"`
	Pid  string `json:"pid"`
}

// ReadGPUStats decodes the output of "intel-gpu-top -J" and iterates through the GPUStats records.
//
// Works with intel-gpu-top v1.17.  If you want to use v1.18 (which uses a different layout), see [V118toV117].
// This middleware converts the output back to v1.17 layout, so it can be processed by ReadGPUStats
func ReadGPUStats(r io.Reader) iter.Seq2[GPUStats, error] {
	return func(yield func(GPUStats, error) bool) {
		dec := json.NewDecoder(r)
		var err error
		for dec.More() {
			var stats GPUStats
			if err = dec.Decode(&stats); err != nil {
				break
			}
			if !yield(stats, nil) {
				return
			}
		}
		if err != nil && !errors.Is(err, io.EOF) {
			yield(GPUStats{}, fmt.Errorf("GetGPUStats: %w", err))
		}
	}
}

var _ io.Reader = &V118toV117{}

// V118toV117 converts the input from v1.18 of intel_gpu_top to v1.17 syntax. Specifically:
//
//   - V1.18 generates the stats as a json array ("[" and "]").
//   - V1.18 *sometimes* (?) writes commas between the stats.
//
// This means json.Decoder will try to read in the full array, where we want to stream the individual records.
// V118toV117 solves this by removed the array & comma tokens, turning the data back to V1.17 layout.
//
// Note: this is *very* dependent on the actual layout of intel_gpu_top's output and will probably break at some point.
type V118toV117 struct {
	Reader io.Reader
}

// Read implements the io.Reader interface
func (v V118toV117) Read(p []byte) (n int, err error) {
	n, err = v.Reader.Read(p)
	if err != nil || n == 0 {
		return n, err
	}
	// note: for ',', this assumes we'll never receive two records in a single read. in practice, this is the case,
	// but may break at some point!
	if p[0] == '[' || p[0] == ',' {
		p[0] = ' '
	}
	if len(p) > 2 && bytes.Equal(p[len(p)-3:], []byte("\n]\n")) {
		n -= 3
	}
	return n, err
}
