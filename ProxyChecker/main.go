package main

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/proxy"
)

const (
	timeout     = 15 * time.Second
	concurrency = 100
)

type Result struct {
	Address string
	Alive   bool
	Latency time.Duration
	Score   int
	Error   error
}

func main() {
	proxies, err := loadProxies("http.txt")
	if err != nil {
		panic(err)
	}

	jobs := make(chan string)
	results := make(chan Result)
	var good []Result

	var wg sync.WaitGroup

	for i := 0; i < concurrency; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for addr := range jobs {
				results <- checkProxy(addr)
			}
		}()
	}

	go func() {
		for _, proxy := range proxies {
			jobs <- proxy
		}

		close(jobs)
		wg.Wait()
		close(results)
	}()

	for r := range results {
		if r.Alive {
			good = append(good, r)
		}
	}

	sort.Slice(good, func(i, j int) bool {
		return good[i].Score > good[j].Score
	})

	for _, r := range good {
		fmt.Printf("%s | score=%d | %v\n", r.Address, r.Score, r.Latency)
	}
}

func loadProxies(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var proxies []string

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" {
			continue
		}

		proxies = append(proxies, line)
	}

	return proxies, scanner.Err()
}

func checkProxy(addr string) Result {
	start := time.Now()
	lat := time.Since(start)

	if _, _, err := net.SplitHostPort(addr); err != nil {
		var result = Result{
			Address: addr,
			Error:   err,
			Alive:   false,
			Latency: 0,
			Score:   0,
		}
		printResult(result)
		return result
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	dialer, err := proxy.SOCKS5(
		"tcp",
		addr,
		nil,
		&net.Dialer{Timeout: 10 * time.Second},
	)

	if err != nil {
		return Result{
			Address: addr,
			Error:   err,
			Alive:   false,
			Latency: 0,
			Score:   0,
		}
	}

	conn, err := dialContext(ctx, dialer, "tcp", "1.1.1.1:80")
	if err != nil {
		return Result{
			Address: addr,
			Error:   err,
			Alive:   false,
			Latency: 0,
			Score:   0,
		}
	}
	defer conn.Close()

	req := "GET / HTTP/1.1\r\nHost: example.com\r\nConnection: close\r\n\r\n"
	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, err = conn.Write([]byte(req))
	if err != nil {
		return Result{
			Address: addr,
			Error:   err,
			Alive:   false,
			Latency: 0,
			Score:   0,
		}
	}

	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		return Result{
			Address: addr,
			Error:   err,
			Alive:   false,
			Latency: 0,
			Score:   0,
		}
	}

	body := string(buf[:n])

	if !strings.Contains(body, "HTTP/") {
		var result = Result{
			Address: addr,
			Error:   fmt.Errorf("invalid HTTP response"),
			Alive:   false,
			Latency: 0,
			Score:   0,
		}
		printResult(result)
		return result
	}

	return Result{
		Address: addr,
		Error:   nil,
		Alive:   true,
		Latency: time.Since(start),
		Score:   calculateScore(true, lat, nil),
	}
}

func dialContext(
	ctx context.Context,
	dialer proxy.Dialer,
	network string,
	addr string,
) (net.Conn, error) {
	type response struct {
		conn net.Conn
		err  error
	}

	ch := make(chan response, 1)

	go func() {
		conn, err := dialer.Dial(network, addr)

		ch <- response{
			conn: conn,
			err:  err,
		}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()

	case r := <-ch:
		return r.conn, r.err
	}
}

func printResult(r Result) {
	fmt.Println("--------------------------------------------------")
	fmt.Printf("Proxy   : %s\n", r.Address)

	if r.Alive {
		fmt.Printf("Status  : ACTIVE ✅\n")
		fmt.Printf("Latency : %v\n", r.Latency)
	} else {
		fmt.Printf("Status  : FAILED ❌\n")
		fmt.Printf("Error   : %v\n", r.Error)
	}
	fmt.Println("--------------------------------------------------")
}

func calculateScore(alive bool, latency time.Duration, err error) int {
	if err != nil || !alive {
		return 0
	}

	score := 20

	if latency < 500*time.Millisecond {
		score += 20
	} else if latency < 1500*time.Millisecond {
		score += 10
	}

	if latency < 300*time.Millisecond {
		score += 10
	}

	return score
}
