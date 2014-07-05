package main

import (
	"errors"
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

// alternate runs a command with alternating parameters inserted in place of the placeholder. Each
// time a SIGUSR1 is received, a new command is run with the next parameter, and a SIGINT is sent to
// the previous command after the overlap duration has elapsed. The alternate logs are written to
// stderr, and the command logs are written to cmdStdout and cmdStderr.
func alternate(command string, placeholder string, params []string, overlap time.Duration,
	stderr, cmdStdout, cmdStderr io.Writer) {

	log.SetPrefix("alternate | ")
	log.SetOutput(stderr)
	log.SetFlags(0)

	log.Printf("Starting with command '%s', placeholder '%s', params = %v, overlap = %vs\n",
		command, placeholder, params, overlap.Seconds())

	// When a command exits, cmdExit receives the parameter this command was run with.
	cmdExit := make(chan string)

	// When the overlap duration has elapsed, overlapEnd receives an empty struct.
	overlapEnd := make(chan struct{})

	next := make(chan os.Signal, 1)
	signal.Notify(next, syscall.SIGUSR1)
	// Buffer a fake USR1 signal to run the first command.
	next <- syscall.SIGUSR1

	m := newManager(params)

	for {
		select {
		case <-testExit:
			log.Println("The test exit channel received a value, exiting alternate")
			return

		case param := <-cmdExit:
			log.Printf("The command with parameter '%s' exited\n", param)
			m.unsetCmd(param)
			if !m.hasCmds() {
				log.Println("All commands have exited, exiting alternate")
				return
			}

		case <-overlapEnd:
			interruptCurrentCmd(m)

		case <-next:
			nextParam := m.nextParam()
			if !m.first() {
				log.Printf("Received signal USR1, rotating to next parameter: '%s'", nextParam)
			}
			if m.nextCmd() != nil {
				log.Printf("A command with parameter '%s' is already running, cannot run again",
					nextParam)
				break
			}

			s := strings.Replace(command, placeholder, nextParam, 1)
			nextCmd := cmd(s, cmdStdout, cmdStderr)
			m.setCmd(nextParam, nextCmd)

			if err := run(nextCmd, cmdExit, nextParam); err != nil {
				log.Printf("Failed to run the command with parameter '%s', error: '%s'\n",
					err.Error())
				m.unsetCmd(nextParam)
				break
			}

			if m.first() {
				m.next()
				break
			}

			if overlap == 0 {
				interruptCurrentCmd(m)
			} else {
				go func() {
					time.Sleep(overlap)
					overlapEnd <- struct{}{}
				}()
			}
		}
	}
}

func interruptCurrentCmd(m *manager) {
	if m.nextCmd() == nil {
		return
	}
	if err := interruptCmd(m.currentCmd()); err == nil {
		m.next()
	}
}

func interruptCmd(c *exec.Cmd) error {
	if c == nil {
		return errors.New("interruptCmd error: cmd is nil")
	}
	p := c.Process
	if p == nil {
		return errors.New("interruptCmd error: cmd.Process is nil")
	}
	p.Signal(os.Interrupt)
	return nil
}

func cmd(s string, stdout, stderr io.Writer) *exec.Cmd {
	c := exec.Command("sh", "-c", s)
	c.Stdout = stdout
	c.Stderr = stderr
	return c
}

func run(c *exec.Cmd, exit chan string, v string) error {
	log.Printf("Running command with parameter '%s'\n", v)
	if err := c.Start(); err != nil {
		return err
	}
	go func() {
		c.Wait()
		exit <- v
	}()
	return nil
}
