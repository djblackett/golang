package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
)

func main() {
	// Start a TCP listener on port 42069
	f, err := net.Listen("tcp", ":42069")
	if err != nil {
		fmt.Println("could not listen on port 42069")
		return
	}

	// Main loop: continually accept new connections
	for {
		conn, err := f.Accept()
		if err != nil {
			fmt.Println("could not accept connection")
			continue
		}
		fmt.Println("accepted connection")

		// Read lines from the connection using a channel-based interface
		ch := getLinesChannel(conn)

		// Print each line received from the channel
		for line := range ch {
			fmt.Printf("read: %s\n", line)
		}
	}

	// Technically unreachable because the for-loop is infinite,
	// but here for completeness in case of refactoring.
	f.Close()
	fmt.Println("closed connection")
}

// getLinesChannel reads bytes from the provided io.ReadCloser (e.g., a TCP connection),
// assembles complete lines (split by '\n'), and sends them to a returned read-only channel.
// It runs in a goroutine so it doesn’t block the caller.
func getLinesChannel(f io.ReadCloser) <-chan string {
	ch := make(chan string) // Output channel for complete lines
	currentLine := ""       // Accumulates data until we hit a newline

	go func() {
		for {
			bytes := make([]byte, 8)        // Read in small chunks (8 bytes at a time)
			n, err := f.Read(bytes)         // Read bytes from connection
			byteString := string(bytes[:n]) // Convert bytes to string

			if !strings.Contains(byteString, "\n") {
				// No newline found, just accumulate more text
				currentLine += byteString
			} else {
				// A newline was found in the chunk — assume only one per chunk
				// (this logic will need improvement if you want to handle more)
				firstSegment := strings.Split(byteString, "\n")[0]  // before newline
				secondSegment := strings.Split(byteString, "\n")[1] // after newline

				// Send complete line to channel and start new line accumulation
				ch <- currentLine + firstSegment
				currentLine = secondSegment
			}

			// Stop reading if connection is closed (EOF)
			if err != nil && errors.Is(err, io.EOF) {
				break
			}
		}

		// If anything remains unflushed, send it as the last line
		if currentLine != "" {
			ch <- currentLine
		}

		close(ch) // Close the channel to signal that reading is done
		f.Close() // Close the connection
	}()

	return ch
}
