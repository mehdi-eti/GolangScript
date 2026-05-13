package main

import (
	"bufio"
	"context"
	"os"
)

func DictionaryWorker(ctx context.Context, filePath string, jobs chan<- string) {
	defer close(jobs)
	file, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		case jobs <- scanner.Text():
		}
	}
}
