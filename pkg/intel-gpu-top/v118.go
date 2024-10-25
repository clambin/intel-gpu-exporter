package intel_gpu_top

import (
	"bytes"
	"io"
)

var _ io.Reader = &V118toV117{}

// V118toV117 converts the input from v1.18 of intel_gpu_top to v1.17 syntax. Specifically:
//
//   - V1.18 generates the stats as a json array, which makes it difficult to stream the data to the reader. V118toV117 removes the array indicators ("[" and "]")
//   - V1.18 *sometimes* (?) writes commas between the stats, meaning the content can't be parsed as individual records.
//
// V118toV117 removes these, turning the data back to V1.17 layout.
//
// Note: this is *very* dependent on the actual layout of intel_gpu_top's output and will probably break at some point.
type V118toV117 struct {
	Reader io.Reader
}

// Read implements the io.Reader interface
func (v *V118toV117) Read(p []byte) (n int, err error) {
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
		p = p[:n]
	}
	return n, err
}
