package kitty

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"
)

func positive(args []string, index, fallback int) (int, error) {
	if len(args) <= index {
		return fallback, nil
	}
	value, err := strconv.Atoi(args[index])
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("positive integer required: %s", args[index])
	}
	return value, nil
}

func Stress(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("stress requires cpu, memory, or network")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	switch args[0] {
	case "cpu":
		workers, err := positive(args, 1, runtime.NumCPU())
		if err != nil {
			return err
		}
		done := ctx.Done()
		var wg sync.WaitGroup
		for range workers {
			wg.Go(func() {
				for {
					select {
					case <-done:
						return
					default:
						runtime.KeepAlive(done)
					}
				}
			})
		}
		fmt.Println("CPU workers active; press Ctrl-C to stop.")
		wg.Wait()
		return nil
	case "memory":
		return stressMemory(ctx, args[1:])
	case "network":
		return stressNetwork(ctx, args[1:])
	default:
		return fmt.Errorf("unknown stress mode: %s", args[0])
	}
}

func stressMemory(ctx context.Context, args []string) error {
	mb, err := positive(args, 0, 64)
	if err != nil {
		return err
	}
	interval := 0.2
	if len(args) > 1 {
		interval, err = strconv.ParseFloat(args[1], 64)
		if err != nil || interval <= 0 || math.IsNaN(interval) || math.IsInf(interval, 0) || interval >= float64(math.MaxInt64)/float64(time.Second) {
			return fmt.Errorf("memory interval is invalid")
		}
	}
	delay := time.Duration(interval * float64(time.Second))
	if delay <= 0 || mb > int(^uint(0)>>1)/(1024*1024) {
		return fmt.Errorf("memory allocation parameters are invalid")
	}
	var chunks [][]byte
	for {
		select {
		case <-ctx.Done():
			runtime.KeepAlive(chunks)
			return nil
		default:
		}
		chunk := make([]byte, mb*1024*1024)
		for i := 0; i < len(chunk); i += 4096 {
			chunk[i] = 1
		}
		chunks = append(chunks, chunk)
		fmt.Printf("allocated: %dMB\n", len(chunks)*mb)
		select {
		case <-ctx.Done():
			runtime.KeepAlive(chunks)
			return nil
		case <-time.After(delay):
		}
	}
}

func stressNetwork(ctx context.Context, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("network requires mode and download/upload URLs")
	}
	mode := args[0]
	if mode != "download" && mode != "upload" && mode != "both" {
		return fmt.Errorf("network mode is invalid")
	}
	if mode == "both" && len(args) < 3 {
		return fmt.Errorf("both requires download and upload URLs")
	}
	for _, endpoint := range args[1:] {
		parsed, err := url.Parse(endpoint)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return fmt.Errorf("network URL is invalid: %s", endpoint)
		}
	}
	mb := 16
	if value := os.Getenv("UPLOAD_MB"); value != "" {
		var err error
		mb, err = positive([]string{value}, 0, 16)
		if err != nil {
			return err
		}
	}
	if mb > int(^uint(0)>>1)/(1024*1024) {
		return fmt.Errorf("upload size is invalid")
	}
	client := &http.Client{Timeout: 30 * time.Second}
	defer client.CloseIdleConnections()
	var wg sync.WaitGroup
	worker := func(method, url string, body []byte) {
		defer wg.Done()
		for ctx.Err() == nil {
			req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return
			}
			response, err := client.Do(req)
			if err == nil {
				_, err = io.Copy(io.Discard, response.Body)
				closeErr := response.Body.Close()
				if err == nil {
					err = closeErr
				}
			}
			if err != nil && ctx.Err() == nil {
				fmt.Fprintln(os.Stderr, err)
				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Second):
				}
			}
		}
	}
	if mode == "download" || mode == "both" {
		wg.Add(1)
		go worker(http.MethodGet, args[1], nil)
	}
	if mode == "upload" || mode == "both" {
		url := args[1]
		if mode == "both" {
			url = args[2]
		}
		wg.Add(1)
		go worker(http.MethodPost, url, make([]byte, mb*1024*1024))
	}
	wg.Wait()
	return nil
}
