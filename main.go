package main

import (
	"errors"
	"fmt"
	"os"
	"time"
)

const (
	placeholder = "%s"
	usage       = `Usage: alternate <command> <parameters> <overlap>

- command: command to run, with %alt used a a placeholder for the rotated parameters. Example: /usr/bin/server 127.0.0.1:%alt
- parameters: space-separated list of parameters to rotate through. Example: 3000 3001
- overlap: delay between starting the next command and sending SIGINT to the previous one. Example: 10s`
)

type arguments struct {
	command string
	params  []string
	overlap time.Duration
}

func main() {
	a, err := args(os.Args)
	if err != nil {
		fmt.Printf("%s\n\n%s\n", err, usage)
		os.Exit(1)
	}

	alternate(a.command, placeholder, a.params, a.overlap, os.Stderr, os.Stdout, os.Stderr)
}

// args parses the command-line arguments.
func args(osArgs []string) (arguments, error) {
	l := len(osArgs)

	if l < 4 {
		return arguments{}, errors.New("Not enough arguments")
	}

	command := osArgs[1]
	params := osArgs[2 : l-1]
	overlapStr := osArgs[l-1]

	overlap, err := time.ParseDuration(overlapStr)
	if err != nil || overlap < 0 {
		return arguments{}, fmt.Errorf("Invalid overlap: '%s'", osArgs[3])
	}

	return arguments{command, params, overlap}, nil
}
