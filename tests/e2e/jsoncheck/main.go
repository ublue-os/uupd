package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

func load(path string) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(path)
}

func lines(path string) int {
	data, err := load(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	count := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if !json.Valid([]byte(line)) {
			fmt.Fprintf(os.Stderr, "invalid JSON: %s\n", line)
			return 1
		}
		count++
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if count == 0 {
		fmt.Fprintln(os.Stderr, "no JSON lines")
		return 1
	}
	return 0
}

func validate(path string) int {
	data, err := load(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: jsoncheck --lines|--validate <file|-> | jsoncheck <file|-> <path> [expected]")
	os.Exit(2)
}

func runMode(args []string) int {
	if len(args) != 2 {
		fmt.Fprintf(os.Stderr, "usage: jsoncheck %s <file|->\n", args[0])
		return 2
	}
	if args[0] == "--lines" {
		return lines(args[1])
	}
	return validate(args[1])
}

func field(value any, path string) (any, error) {
	for _, key := range strings.Split(path, ".") {
		object, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("not an object at: %s", key)
		}
		value, ok = object[key]
		if !ok {
			return nil, fmt.Errorf("missing key: %s", key)
		}
	}
	return value, nil
}

func text(value any) string {
	switch v := value.(type) {
	case bool:
		return fmt.Sprintf("%t", v)
	case string:
		return v
	case nil:
		return "null"
	default:
		return fmt.Sprint(value)
	}
}

func query(args []string) int {
	if len(args) < 2 {
		usage()
	}
	data, err := load(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	value, err = field(value, args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	result := text(value)
	if len(args) == 3 && result != args[2] {
		fmt.Fprintf(os.Stderr, "expected %q, got %q\n", args[2], result)
		return 1
	}
	fmt.Println(result)
	return 0
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		usage()
	}
	if args[0] == "--lines" || args[0] == "--validate" {
		os.Exit(runMode(args))
	}
	os.Exit(query(args))
}
