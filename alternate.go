package main

import (
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// testExit is a channel that is used during testing only to trigger a return from the alternate
// function.
var testExit chan struct{}

// alternate runs a command with alternating values inserted in place of the placeholder. Each time
// a SIGUSR1 is received, a new command is run with the next value, and the previous command is sent
// a SIGINT after the overlap duration has elapsed. The alternate logs are written to stderr, and
// the command logs are written to cmdStdout and cmdStderr;
func alternate(command string, placeholder string, values []string, overlap time.Duration,
	stderr, cmdStdout, cmdStderr io.Writer) {

	log.SetPrefix("alternate | ")
	log.SetOutput(stderr)
	log.SetFlags(0)

	log.Printf("Starting with command = '%s', placeholder '%s', values = %v, overlap = %vs\n",
		command, placeholder, values, overlap.Seconds())

	// next receives a signal when the next command should be run.
	next := make(chan os.Signal, 1)
	signal.Notify(next, syscall.SIGUSR1)
	// Run the first command.
	next <- syscall.Signal(0)

	// overlapEnd receives an empty struct when the overlap duration has elapsed.
	overlapEnd := make(chan struct{})

	// cmdExit receives the value associated with a command when the command exits.
	cmdExit := make(chan string)

	m := newManager(values)

	for {
		select {
		case <-testExit:
			log.Println("testExit channel triggered, exiting alternate")
			return

		case value := <-cmdExit:
			log.Printf("Command with value '%s' exited\n", value)
			m.unsetCmd(value)
			if !m.hasCmds() {
				log.Println("No running commands, exiting")
				return
			}

		case signal := <-next:
			nextValue := m.nextValue()

			if signal == syscall.SIGUSR1 {
				log.Printf("Received USR1, trying to move to next value: '%s'", nextValue)
			}

			if m.nextCmd() != nil {
				log.Printf("Command with value '%s' still running, cannot run again\n", nextValue)
				break
			}

			s := strings.Replace(command, placeholder, nextValue, 1)
			nextCmd := cmd(s, cmdStdout, cmdStderr)
			m.setCmd(nextValue, nextCmd)

			if err := run(nextCmd, cmdExit, nextValue); err != nil {
				log.Printf("Command with value '%s' failed to run, error: '%s'\n", err.Error())
				m.unsetCmd(nextValue)
				break
			}

			if m.first() {
				m.next()
				break
			}

			if overlap == 0 {
				if m.nextCmd() != nil && interruptCmd(m.currentCmd()) {
					m.next()
				}
			} else {
				go func() {
					time.Sleep(overlap)
					overlapEnd <- struct{}{}
				}()
			}

		case <-overlapEnd:
			if m.nextCmd() != nil && interruptCmd(m.currentCmd()) {
				m.next()
			}
		}
	}
}

func interruptCmd(c *exec.Cmd) bool {
	if c == nil {
		return false
	}
	p := c.Process
	if p == nil {
		return false
	}
	p.Signal(os.Interrupt)
	return true
}

func cmd(s string, stdout, stderr io.Writer) *exec.Cmd {
	c := exec.Command("sh", "-c", s)
	c.Stdout = stdout
	c.Stderr = stderr
	return c
}

func run(c *exec.Cmd, exit chan string, v string) error {
	log.Printf("Running command with value '%s'\n", v)
	if err := c.Start(); err != nil {
		return err
	}
	go func() {
		c.Wait()
		exit <- v
	}()
	return nil
}
