package intel_gpu_top

import (
	"bytes"
	"errors"
	"io"
)

// V118ArrayRemover converts the input from v1.18 of intel_gpu_top to v1.17 syntax.  Specifically,
// v1.18 wraps the stats into a json array ("[" and "]").  V118ArrayRemover removes the leading and
// trailing array markers, so we can pass a stream of stat objects to json.Decoder.
type V118ArrayRemover struct {
	Reader       io.Reader
	arrayRemoved bool
}

func (r *V118ArrayRemover) Read(p []byte) (n int, err error) {
	tmp := make([]byte, len(p))
	n, err = r.Reader.Read(tmp)
	if err != nil && !errors.Is(err, io.EOF) {
		return n, err
	}
	if n > 0 {
		tmp = tmp[:n]
		if !r.arrayRemoved {
			// TODO: better: is the first non-white space character a '['?
			if index := bytes.Index(tmp, []byte("[")); index >= 0 {
				tmp = tmp[index+1:]
				n -= index + 1
				r.arrayRemoved = true
			}
		}
		// TODO: if err == io.EOF, remove trailing ']'
	}
	copy(p, tmp)
	return n, err
}
