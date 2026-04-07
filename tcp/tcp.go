package main

import (
	"fmt"
	"net"
	"time"
)

func checkServer(address string) {
	timeout := 2 * time.Second

	conn, err := net.DialTimeout("tcp", address, timeout)

	if err != nil {
		fmt.Println("Error !!!", err.Error())
		return
	}

	fmt.Println("address", address)
	conn.Close()
}

func main() {
	checkServer("github.com:443")
}
