package requests

import (
	"errors"
	"io"
	"strings"
	"unicode"
)

// chatGPT added all of the comments because I took a month off from this course and I couldn't
// remember the purpose of the code I wrote earlier

type RequestState int

const bufferSize = 8

const (
	Initialized RequestState = iota
	Done
)

// Request represents a parsed HTTP request, including the request line.
type Request struct {
	RequestLine RequestLine
	State       RequestState
}

// RequestLine holds the components of an HTTP request line:
// e.g., "GET /index.html HTTP/1.1"
type RequestLine struct {
	HttpVersion   string // e.g., "1.1"
	RequestTarget string // e.g., "/index.html"
	Method        string // e.g., "GET"
}

// RequestFromReader manually parses an HTTP request from an io.Reader.
// It currently only parses the request line (the first line of an HTTP request).
func RequestFromReader(reader io.Reader) (*Request, error) {

	buf := make([]byte, bufferSize)
	var readToIndex = 0

	request := Request{
		State: Initialized,
	}

	for {

		if request.State == Done {
			break
		}

		if readToIndex == len(buf) {
			temp := make([]byte, len(buf)*2)
			copy(temp, buf)
			buf = temp
		}

		byteCount, err := reader.Read(buf[readToIndex:])
		readToIndex += byteCount

		if err != nil {
			if err == io.EOF {

				request.State = Done
			} else {
				return nil, err
			}
		}

		count, err := request.parse(buf[:readToIndex])
		if err != nil {
			return nil, err
		}

		if count == 0 {
			continue
		}

		copy(buf, buf[count:readToIndex])
		readToIndex -= count

	}

	return &request, nil
}

func parseRequestLine(text []byte) (RequestLine, int, error) {

	// Split the request into lines using CRLF as per HTTP/1.1 spec.
	textString := string(text)
	if !strings.Contains(textString, "\r\n") {
		return RequestLine{}, 0, nil
	}
	lines := strings.Split(textString, "\r\n")

	// Extract the request line: e.g., "GET /index.html HTTP/1.1"
	requestLine := lines[0]
	byteCount := len([]byte(requestLine)) + 2 // +2 for \r\n

	// Split the request line into three parts: METHOD, TARGET, VERSION
	parts := strings.Split(requestLine, " ")
	if len(parts) != 3 {
		return RequestLine{}, 0, errors.New("Request line does not have enough parts")
	}

	method := parts[0]
	target := parts[1]
	versionPart := parts[2]

	// Validate that the HTTP version is supported (only HTTP/1.1 allowed for now)
	if versionPart != "HTTP/1.1" {
		return RequestLine{}, 0, errors.ErrUnsupported
	}

	// Ensure method is uppercase (e.g., "GET")
	if !IsUpper(method) {
		return RequestLine{}, 0, errors.ErrUnsupported
	}

	version := strings.Split(versionPart, "/")[1]

	return RequestLine{
		HttpVersion:   version,
		RequestTarget: target,
		Method:        method,
	}, byteCount, nil
}

// IsUpper checks if all letters in the string are uppercase.
// This is used to validate the HTTP method (e.g., "GET", "POST").
func IsUpper(s string) bool {
	for _, r := range s {
		if !unicode.IsUpper(r) && unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

func (r *Request) parse(data []byte) (int, error) {
	switch r.State {
	case Initialized:
		requestLine, count, err := parseRequestLine(data)
		if count == 0 {
			return 0, nil
		}
		if err != nil {
			return 0, err
		}
		r.RequestLine = requestLine
		r.State = Done
		return count, nil
	case Done:
		return 0, errors.New("error: trying to read data in a done state")
	default:
		return 0, errors.New("error: unknown state")
	}
}
