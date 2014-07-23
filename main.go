package main

import (
	"errors"
	"fmt"
	"os"
	"time"
)

const (
	placeholder = "%alt"
	usage       = `Usage: alternate <command> <parameters...> <overlap>

- command: command to run, with the substring ` + placeholder + ` used a a placeholder for the rotated parameters.
- parameters: space-separated list of parameters to rotate through after receiving a USR1 signal.
- overlap: delay between starting the next command and sending a TERM signal to the previous command.

Example: alternate "/home/me/myserver 127.0.0.1:%alt" 3000 3001 15s

See https://github.com/peferron/alternate for more information.`
)

type arguments struct {
	command string
	params  []string
	overlap time.Duration
}

func main() {
	a, err := parseArguments(os.Args)
	if err != nil {
		fmt.Printf("%v\n\n%s\n", err, usage)
		os.Exit(1)
	}

	alternate(a.command, placeholder, a.params, a.overlap, os.Stderr, os.Stdout, os.Stderr)
}

func parseArguments(osArgs []string) (arguments, error) {
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
