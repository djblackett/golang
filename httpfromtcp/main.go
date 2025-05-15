package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
)

func main() {
	// f, err := os.Open("./messages.txt")
	// if err != nil {
	// 	fmt.Println("could not open file")
	// }
	f, err := net.Listen("tcp", ":42069")
	if err != nil {
		fmt.Println("could not listen on port 42069")
		return
	}

	ch := getLinesChannel(f)

	for line := range ch {
		fmt.Printf("read: %s\n", line)
	}
}

func getLinesChannel(f io.ReadCloser) <-chan string {
	ch := make(chan string)
	currentLine := ""

	go func() {
		for {

			bytes := make([]byte, 8)
			n, err := f.Read(bytes)
			byteString := string(bytes[:n])
			if !strings.Contains(byteString, "\n") {
				currentLine += byteString
			} else {
				firstSegment := strings.Split(byteString, "\n")[0]
				secondSegment := strings.Split(byteString, "\n")[1]
				ch <- currentLine + firstSegment
				currentLine = secondSegment

			}

			if err != nil && errors.Is(err, io.EOF) {
				break
			}
		}
		if currentLine != "" {
			ch <- currentLine
		}
		close(ch)
		f.Close()
	}()
	return ch
}
