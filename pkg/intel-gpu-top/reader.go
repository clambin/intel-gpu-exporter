package intel_gpu_top

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
)

type GPUStats struct {
	Period struct {
		Duration float64 `json:"duration"`
		Unit     string  `json:"unit"`
	} `json:"period"`
	Frequency struct {
		Requested float64 `json:"requested"`
		Actual    float64 `json:"actual"`
		Unit      string  `json:"unit"`
	} `json:"frequency"`
	Interrupts struct {
		Count float64 `json:"count"`
		Unit  string  `json:"unit"`
	} `json:"interrupts"`
	Rc6 struct {
		Value float64 `json:"value"`
		Unit  string  `json:"unit"`
	} `json:"rc6"`
	Power struct {
		GPU     float64 `json:"GPU"`
		Package float64 `json:"Package"`
		Unit    string  `json:"unit"`
	} `json:"power"`
	ImcBandwidth struct {
		Reads  float64 `json:"reads"`
		Writes float64 `json:"writes"`
		Unit   string  `json:"unit"`
	} `json:"imc-bandwidth"`
	Engines map[string]EngineStats `json:"engines"`
	Clients map[string]Client      `json:"clients"`
}

type EngineStats struct {
	Busy float64 `json:"busy"`
	Sema float64 `json:"sema"`
	Wait float64 `json:"wait"`
	Unit string  `json:"unit"`
}

type Client struct {
	Name          string `json:"name"`
	Pid           string `json:"pid"`
	EngineClasses map[string]struct {
		Busy string `json:"busy"`
		Unit string `json:"unit"`
	} `json:"engine-classes"`
}

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
