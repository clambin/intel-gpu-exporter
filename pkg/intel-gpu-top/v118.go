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
	tmp := make([]byte, len(p))
	n, err = v.Reader.Read(tmp)
	if err != nil || n == 0 {
		return n, err
	}
	if tmp[0] == '[' || tmp[0] == ',' {
		tmp[0] = ' '
	}
	if index := bytes.LastIndex(tmp, []byte("\n]\n")); index >= 0 {
		n = index + 1
		tmp = tmp[:n]
	}
	copy(p, tmp[:n])

	return n, err
}
