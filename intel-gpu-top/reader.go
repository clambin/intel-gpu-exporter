package intel_gpu_top

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"strconv"
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
	Memory map[string]map[string]MemoryBytes `json:"memory"`
	Name   string                            `json:"name"`
	Pid    string                            `json:"pid"`
}

type MemoryBytes uint64

func (m *MemoryBytes) UnmarshalJSON(b []byte) error {
	var u uint64
	b = bytes.Trim(b, "\"")
	if err := json.Unmarshal(b, &u); err != nil {
		return err
	}
	*m = MemoryBytes(u)
	return nil
}

func (m MemoryBytes) MarshalJSON() ([]byte, error) {
	return []byte(`"` + strconv.FormatUint(uint64(m), 10) + `"`), nil
}

// ReadGPUStats decodes the output of "intel-gpu-top -J" and iterates through the GPUStats records.
//
// JSON output of "intel-gpu-top -J" is a bit funky": it's a stream of JSON objects (that may or may not
// have separating comma between the objects), but also contains an outer array. For example:
//
//	[
//	  { ... }
//	  { ... }
//	  ...
//	]
//
// The stdlib JSON library rejects this. This function addresses this by stripping the outer array
// and removing the separating comma's, so that we can treat the output as a stream of JSON objects.
//
// Works with intel-gpu-top v2.3.
func ReadGPUStats(r io.Reader) iter.Seq2[GPUStats, error] {
	return func(yield func(GPUStats, error) bool) {
		dec := json.NewDecoder(&jsonFormatter{Source: r})
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
			yield(GPUStats{}, fmt.Errorf("json: %w", err))
		}
	}
}

// jsonFormatter is a io.Reader that strips the outer array and removes the separating commas
// from intel_gpu_top's JSON output.
type jsonFormatter struct {
	Source     io.Reader
	out        bytes.Buffer
	depth      int
	buf        [512]byte // allocate here to reduce allocations in Read()
	seenStart  bool
	inString   bool
	escapeNext bool
}

func (j *jsonFormatter) Read(p []byte) (int, error) {
	// Serve buffered output first
	if j.out.Len() > 0 {
		return j.out.Read(p)
	}

	for {
		// read more data
		n, err := j.Source.Read(j.buf[:])
		if n > 0 {
			j.process(j.buf[:n])
		}

		// Serve processes output
		if j.out.Len() > 0 {
			return j.out.Read(p)
		}

		// if we hit an error (which may be EOF), return it
		if err != nil {
			return 0, err
		}
	}
}

func (j *jsonFormatter) process(p []byte) {
	for _, b := range p {
		// Handle strings and escape sequences
		if j.inString {
			j.out.WriteByte(b)

			if j.escapeNext {
				j.escapeNext = false
			} else if b == '\\' {
				j.escapeNext = true
			} else if b == '"' {
				j.inString = false
			}
			continue
		}

		// check for array start/end, JSON objects and strings
		switch b {
		case '"':
			j.inString = true
		case '{', '[':
			// start of an array or object: strip it only if it's the root array
			if b == '[' && !j.seenStart && j.depth == 0 {
				j.seenStart = true
				j.depth++
				continue
			}
			j.depth++
		case '}', ']':
			// end of an array or object: strip it only if it's the root array
			if j.depth > 0 {
				j.depth--
			}
			if b == ']' && j.seenStart && j.depth == 0 {
				continue
			}
		case ',':
			// skip commas at level 1 (i.e., the level inside the root array)
			if j.depth == 1 {
				continue
			}
		}
		j.out.WriteByte(b)
	}
}
