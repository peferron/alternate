package main

import (
	"errors"
	"fmt"
	"os"

	"time"
)

const (
	usage = `Usage: alternate <command> <values> <overlap>

- command: command to run, with %alternate used a a placeholder for the rotated values. Example: /usr/bin/server 127.0.0.1:%alternate
- values: space-separated list of values to rotate through. Example: 3000 3001
- overlap: delay between starting the next command and sending SIGINT to the previous one. Example: 10s`
)

type arguments struct {
	command string
	values  []string
	overlap time.Duration
}

func main() {
	if l := len(os.Args); l == 1 || l == 2 && os.Args[1] == "help" {
		fmt.Println(usage)
		os.Exit(2)
	}

	a, err := args(os.Args)
	if err != nil {
		fmt.Println(err.Error(), "", "Run 'alternate help' for usage.")
		os.Exit(1)
	}

	alternate(a, os.Stderr, os.Stdout, os.Stderr)
}

// args parses the command-line arguments.
func args(a []string) (arguments, error) {
	l := len(a)
	if l < 4 {
		return arguments{}, errors.New("Not enough arguments")
	}

	command := a[1]

	values := a[2 : l-2]

	overlapStr := a[l-1]
	overlap, err := time.ParseDuration(overlapStr)
	if err != nil {
		return arguments{}, fmt.Errorf("Invalid overlap: %s", os.Args[3])
	}

	return arguments{command, values, overlap}, nil
}
