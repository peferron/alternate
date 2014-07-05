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

	log.Printf("Starting with command %q, placeholder %q, params = %q, overlap = %v\n",
		command, placeholder, params, overlap)

	// When a command exits, cmdExit receives the parameter this command was run with.
	cmdExit := make(chan string)

	// When the overlap duration has elapsed, overlapEnd receives an empty struct.
	overlapEnd := make(chan struct{})

	next := make(chan os.Signal, 1)
	// Buffer a fake signal to run the first command.
	next <- syscall.Signal(0)
	// Then listen to USR1 signals dispatched by the user.
	signal.Notify(next, syscall.SIGUSR1)

	m := newManager(params)

	for {
		select {
		case <-testExit:
			log.Println("Test exit channel received a value, exiting alternate")
			killAllCmds(params, m)
			return

		case param := <-cmdExit:
			log.Printf("Command with parameter %q exited\n", param)
			m.unsetCmd(param)
			if !m.hasCmds() {
				log.Println("All commands have exited, exiting alternate")
				return
			}

		case <-overlapEnd:
			finishRotation(m)

		case signal := <-next:
			first := signal != syscall.SIGUSR1
			nextParam := m.nextParam()

			if !first {
				log.Printf("Received signal USR1, rotating to next parameter %q", nextParam)
			}

			if m.nextCmd() != nil {
				log.Printf("A command with parameter %q is already running, cannot run again",
					nextParam)
				break
			}

			s := strings.Replace(command, placeholder, nextParam, 1)
			nextCmd := cmd(s, cmdStdout, cmdStderr)
			m.setCmd(nextParam, nextCmd)

			if err := run(nextCmd, nextParam, cmdExit); err != nil {
				log.Printf("Failed to run the command with parameter %q, error: %v\n",
					err.Error())
				m.unsetCmd(nextParam)
				break
			}

			if first {
				// There is no previous command to interrupt.
				m.rotate()
				break
			}

			if overlap == 0 {
				finishRotation(m)
			} else {
				log.Printf("Waiting %v before sending interrupt to command with parameter %q\n",
					overlap, m.currentParam())
				go func() {
					time.Sleep(overlap)
					overlapEnd <- struct{}{}
				}()
			}
		}
	}
}

func finishRotation(m *manager) {
	if m.nextCmd() == nil {
		// The next command is not running, cancel the rotation without interrupting the current
		// command.
		return
	}
	if c := m.currentCmd(); c != nil {
		currentParam := m.currentParam()
		log.Printf("Sending interrupt to command with parameter %q\n", currentParam)
		if err := interruptCmd(c); err != nil {
			log.Printf("Failed to send interrupt to command with parameter %q, error: %v\n",
				currentParam, err)
		}
	}
	m.rotate()
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
	f := strings.Fields(s)
	c := exec.Command(f[0], f[1:]...)
	c.Stdout = stdout
	c.Stderr = stderr
	return c
}

func run(c *exec.Cmd, param string, exit chan string) error {
	log.Printf("Running command with parameter %q\n", param)
	if err := c.Start(); err != nil {
		return err
	}
	go func() {
		c.Wait()
		exit <- param
	}()
	return nil
}

func killAllCmds(params []string, m *manager) {
	for _, param := range params {
		if c := m.cmd(param); c != nil {
			if p := c.Process; p != nil {
				log.Printf("Killing command with parameter %q\n", param)
				p.Signal(os.Kill)
			}
		}
	}
}
