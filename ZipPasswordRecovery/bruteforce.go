package main

import (
	"context"
)

func BruteForceWorker(ctx context.Context, charset string, minLen, maxLen int, jobs chan<- string) {
	defer close(jobs)

	// Iterative approach to generate combinations
	for length := minLen; length <= maxLen; length++ {
		indices := make([]int, length)
		for {
			password := make([]byte, length)
			for i, idx := range indices {
				password[i] = charset[idx]
			}

			select {
			case <-ctx.Done():
				return
			case jobs <- string(password):
			}

			// Increment indices
			i := length - 1
			for i >= 0 {
				indices[i]++
				if indices[i] < len(charset) {
					break
				}
				indices[i] = 0
				i--
			}
			if i < 0 {
				break
			}
		}
	}
}
