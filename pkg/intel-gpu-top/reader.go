package intel_gpu_top

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"strings"
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
type V118toV117 struct {
	Source io.Reader
	output bytes.Buffer
	jsonTracker
	buffer [512]byte // json reads in 512 blocks
}

// Read reads from the source and extracts complete JSON objects.
func (r *V118toV117) Read(p []byte) (n int, err error) {
	if r.output.Len() > 0 {
		return r.output.Read(p)
	}

	// don't allocate a buffer on every read
	buf := r.buffer[:]
	for r.output.Len() == 0 {
		clear(buf)
		if n, err = r.Source.Read(buf); err == io.EOF {
			break
		}
		if err != nil {
			return 0, err
		}
		// run each byte through jsonTracker. when we've collected a complete JSON object,
		// add it to r.output.
		for _, char := range buf[:n] {
			// skip any [, ] or , at root level. This turns the stream into a V117-compliant structure.
			if r.jsonTracker.atRootLevel() && strings.IndexByte("[],", char) != -1 {
				continue
			}
			r.jsonTracker.Process(char)

			// If a complete JSON object is detected, add it to r.output.
			// r.output.WriteTo empties jsonTracker's buffer.
			if obj, ok := r.jsonTracker.HasCompleteObject(); ok {
				_, _ = obj.WriteTo(&r.output)
			}
		}
	}
	return r.output.Read(p)
}

// jsonTracker is a helper for V118toV117 that reads in json data and works out when we've received a complete json object.
type jsonTracker struct {
	buffer       bytes.Buffer
	nestingLevel int
	inString     bool
	escapeNext   bool
}

func (r *jsonTracker) Process(char byte) {
	r.buffer.WriteByte(char)
	if r.inString {
		if r.escapeNext {
			r.escapeNext = false
		} else if char == '\\' {
			r.escapeNext = true
		} else if char == '"' {
			r.inString = false
		}
	} else {
		switch char {
		case '{':
			r.nestingLevel++
		case '}':
			r.nestingLevel--
		case '"':
			r.inString = true
		}
	}
}

func (r *jsonTracker) atRootLevel() bool {
	return r.nestingLevel == 0 && !r.inString
}

func (r *jsonTracker) HasCompleteObject() (*bytes.Buffer, bool) {
	if r.atRootLevel() && r.buffer.Len() > 0 {
		return &r.buffer, true
	}
	return nil, false
}
