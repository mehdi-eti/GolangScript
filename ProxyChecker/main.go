package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	file, _ := os.Open("http.txt")
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var ip string
		var port int

		if strings.Contains(line, "{") && strings.Contains(line, "}") {
			line = strings.Trim(line, "{}")
			parts := strings.Split(line, ",")
			ip = strings.Trim(strings.TrimSpace(parts[0]), "\"")
			fmt.Sscanf(strings.TrimSpace(parts[1]), "%d", &port)
		} else if strings.Contains(line, ":") {
			parts := strings.Split(line, ":")
			ip = parts[0]
			port, _ = strconv.Atoi(parts[1])
		} else {
			fmt.Sscanf(line, "%s %d", &ip, &port)
		}

		if ip != "" && port > 0 {
			fmt.Printf("%s:%d\n", ip, port)
			response := testSOCKS5Proxy(ip, port)
			if response {
				fmt.Println("------------------------------")
				fmt.Printf("✓ %s:%d - Active ✅\n", ip, port)
				fmt.Println("------------------------------")
			}
		}
	}
	fmt.Println("Done ✅")
}

func testSOCKS5Proxy(ip string, port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", ip, port), 5*time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(5 * time.Second))

	_, err = conn.Write([]byte{0x05, 0x01, 0x00})
	if err != nil {
		return false
	}

	resp := make([]byte, 2)
	_, err = conn.Read(resp)
	if err != nil {
		return false
	}

	return resp[0] == 0x05 && resp[1] == 0x00
}
