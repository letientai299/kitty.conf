package tabbar

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestServe(t *testing.T) {
	t.Setenv("KITTY_REPO_STATE", t.TempDir())
	input := `{"version":1,"revision":3,"complete":true,"tabs":[{"id":4,"title":"zsh","cwd":"","manual_title":""}]}
{"version":2,"revision":4}
not json
`
	var out bytes.Buffer
	if err := Serve(strings.NewReader(input), &out); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("replies: %q", out.String())
	}
	var first Response
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatal(err)
	}
	if first.Revision != 3 || first.Error != "" || first.Tabs[4].Active.Segments[1].Text != "zsh" {
		t.Fatalf("first reply: %+v", first)
	}
	for _, line := range lines[1:] {
		var reply Response
		if err := json.Unmarshal([]byte(line), &reply); err != nil || reply.Error == "" || reply.Version != Version {
			t.Fatalf("error reply: %s", line)
		}
	}
	if err := Serve(strings.NewReader(strings.Repeat("x", MaxMessage+1)+"\n"), &out); err == nil {
		t.Fatal("oversized request accepted")
	}
}
