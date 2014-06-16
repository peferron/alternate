package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

const (
	sign = syscall.SIGUSR2
)

type options struct {
	server    string
	addresses []string
	overlap   time.Duration
	env       []string
}

func main() {
	log.SetPrefix("alt      | ")

	o, err := opts()
	if err != nil {
		log.Println(err.Error())
		os.Exit(1)
	}

	log.Printf("Options:\n- Server: %s\n- Addresses: %s\n- Overlap: %v\n- Env: %s\n",
		o.server, strings.Join(o.addresses, " "), o.overlap, strings.Join(o.env, " "))

	ch := make(chan os.Signal)
	signal.Notify(ch, sign)

	address := ""
	concurrent := 0
	max := len(o.addresses)

	var cmd *exec.Cmd
	var prevCmd *exec.Cmd

	term := make(chan os.Signal)
	signal.Notify(term, syscall.SIGTERM)
	go func() {
		<-term
		log.Printf("Received signal '%v', exiting now\n", syscall.SIGTERM)
		if cmd != nil {
			cmd.Process.Kill()
		}
		if prevCmd != nil {
			prevCmd.Process.Kill()
		}
		os.Exit(0)
	}()

	for {
		address = nextAddress(o.addresses, address)
		cmd = exec.Command(o.server, address)
		cmd.Env = o.env

		go func(address string) {
			log.Printf("Starting new server instance at address %s, %d currently running\n",
				address, concurrent)

			defer func() {
				log.Printf("Server instance at address %s exited", address)
				concurrent--
			}()
			concurrent++

			run(cmd)
		}(address)

		log.Printf("Waiting for signal '%v'\n", sign)
		<-ch
		for concurrent >= max {
			log.Printf("Ignoring signal '%v', %d server instances already running\n", sign, concurrent)
			<-ch
		}

		go forward(cmd, o.overlap, sign)

		prevCmd = cmd
	}
}

// opts parses the command-line options.
func opts() (options, error) {
	e := func(msg string) error {
		return fmt.Errorf(`Usage: alternate <server> <addresses> <overlap> <env>
- server: path to the server to run. Example: /usr/bin/server
- addresses: comma-separated list of addresses to pass as an argument to the server. Example: :3000,:3001
- overlap: overlap duration between the two server instances. Example: 10s
- env: comma-separated list of environment variables to forward. Example: VAR_1,VAR_2
%s`, msg)
	}

	o := options{}

	if len(os.Args) < 5 {
		return o, e("Missing arguments")
	}

	o.server = os.Args[1]

	o.addresses = strings.Split(os.Args[2], ",")

	if overlap, err := time.ParseDuration(os.Args[3]); err != nil {
		return o, e(fmt.Sprintf("Invalid overlap: %s", os.Args[3]))
	} else {
		o.overlap = overlap
	}

	o.env = getEnv(strings.Split(os.Args[4], ","))

	return o, nil
}

// getEnv builds the list of environment variables with their values.
func getEnv(keys []string) []string {
	env := []string{}
	for _, k := range keys {
		v := os.Getenv(k)
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	return env
}

// forward forwards the signal to the process of the given command once the delay is reached.
func forward(cmd *exec.Cmd, delay time.Duration, sign os.Signal) {
	address := cmd.Args[1]

	log.Printf("Received signal '%v', will forward to server instance at address %s after %vs\n",
		sign, address, delay.Seconds())

	time.Sleep(delay)

	log.Printf("Forwarding signal '%v' to server instance at address %s\n", sign, address)
	cmd.Process.Signal(sign)
}

// Run executes the given command and copies its stdout and stderr.
func run(cmd *exec.Cmd) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		panic(err)
	}

	err = cmd.Start()
	if err != nil {
		panic(err)
	}

	go io.Copy(os.Stdout, stdout)
	go io.Copy(os.Stderr, stderr)

	cmd.Wait()
}

// nextAddress returns the next address that should be used for listening.
func nextAddress(addresses []string, address string) string {
	i := indexOf(addresses, address) + 1
	if i >= len(addresses) {
		i = 0
	}
	return addresses[i]
}

// indexOf returns the index of the given value in the given slice, or -1 if it cannot be found.
func indexOf(slice []string, value string) int {
	for p, v := range slice {
		if v == value {
			return p
		}
	}
	return -1
}
