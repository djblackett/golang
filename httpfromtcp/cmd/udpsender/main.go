package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
)

func main() {
	remoteAddr, err := net.ResolveUDPAddr("udp", ":42069")
	if err != nil {
		panic(err)
	}
	conn, err := net.DialUDP("udp", nil, remoteAddr)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	bufioReader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("> ")
		line, err := bufioReader.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading from stdin:", err)
			break
		}
		if line == "exit\n" {
			break
		}
		n, err := conn.Write([]byte(line))
		if err != nil {
			fmt.Println("Error writing to UDP connection:", err)
			break
		}
		fmt.Printf("Sent %d bytes: %s", n, line)
	}
}
