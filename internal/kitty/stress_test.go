package kitty

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestStressInvalid(t *testing.T) {
	for _, args := range [][]string{{"both", "https://example.com", "bad"}, {"download", "file:///tmp/data"}, {"upload", "http://"}} {
		if err := stressNetwork(context.Background(), args); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	for _, interval := range []string{"NaN", "+Inf", "1e100", "-1", "0"} {
		if err := stressMemory(context.Background(), []string{"1", interval}); err == nil {
			t.Fatalf("accepted %s", interval)
		}
	}
}

func TestNetworkUpload(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	received := make(chan int64, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n, err := io.Copy(io.Discard, r.Body)
		if err != nil {
			t.Error(err)
		}
		select {
		case received <- n:
		default:
		}
		cancel()
	}))
	defer server.Close()
	t.Setenv("UPLOAD_MB", "1")
	if err := stressNetwork(ctx, []string{"upload", server.URL}); err != nil {
		t.Fatal(err)
	}
	select {
	case n := <-received:
		if n != 1024*1024 {
			t.Fatalf("upload bytes: %d", n)
		}
	default:
		t.Fatal("upload missing")
	}
}
