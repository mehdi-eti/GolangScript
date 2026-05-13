package main

import (
	"context"
	"sync"
	"sync/atomic"
)

func Worker(ctx context.Context, zipPath string, jobs <-chan string, found chan<- string, wg *sync.WaitGroup, attempts *uint64) {
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case password, ok := <-jobs:
			if !ok {
				return
			}
			atomic.AddUint64(attempts, 1)
			if ok, _ := CheckPassword(zipPath, password); ok {
				select {
				case found <- password:
				default:
				}
				return
			}
		}
	}
}
