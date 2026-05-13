package main

import (
	"context"
	"flag"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	zipPath := flag.String("zip", "", "Path to ZIP file")
	dictPath := flag.String("dict", "", "Path to dictionary file (optional)")
	charset := flag.String("charset", "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789", "Brute force charset")
	minLen := flag.Int("min", 1, "Min length")
	maxLen := flag.Int("max", 4, "Max length")
	workers := flag.Int("threads", 8, "Number of threads")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	jobs := make(chan string, *workers*2)
	found := make(chan string, 1)
	var wg sync.WaitGroup
	var attempts uint64

	// Start Stats Reporter
	go func() {
		start := time.Now()
		for {
			select {
			case <-time.After(1 * time.Second):
				fmt.Printf("Attempts: %d | Rate: %d/s | Time: %v\n",
					atomic.LoadUint64(&attempts), atomic.LoadUint64(&attempts)/uint64(time.Since(start).Seconds()+1), time.Since(start).Round(time.Second))
			case <-ctx.Done():
				return
			}
		}
	}()

	// Start Workers
	for i := 0; i < *workers; i++ {
		wg.Add(1)
		go Worker(ctx, *zipPath, jobs, found, &wg, &attempts)
	}

	// Start Producer
	if *dictPath != "" {
		go DictionaryWorker(ctx, *dictPath, jobs)
	} else {
		go BruteForceWorker(ctx, *charset, *minLen, *maxLen, jobs)
	}

	// Result Waiter
	select {
	case pass := <-found:
		fmt.Printf("\nSUCCESS! Password found: %s\n", pass)
		cancel()
	case <-time.After(100 * time.Hour): // Placeholder for endless wait
	}
	wg.Wait()
}
