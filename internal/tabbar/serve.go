package tabbar

import (
	"bufio"
	"encoding/json"
	"io"
)

// MaxMessage bounds one newline-delimited request.
const MaxMessage = 1 << 20

// Serve answers one JSON response per request line until EOF.
func Serve(r io.Reader, w io.Writer) error {
	b := NewBuilder()
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), MaxMessage)
	encoder := json.NewEncoder(w)
	for scanner.Scan() {
		var req Request
		var out Response
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			out = Response{Version: Version, Error: "snapshot decode failed: " + err.Error()}
		} else if out, err = b.Render(req); err != nil {
			out = Response{Version: Version, Revision: req.Revision, Error: err.Error()}
		}
		if err := encoder.Encode(out); err != nil {
			return err
		}
	}
	return scanner.Err()
}
