package main

import (
	"errors"
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

type runFunc func(param string) (*exec.Cmd, error)

// testKill is a channel that is used during testing only to trigger an immediate cleanup and return
// from the alternate function.
var testKill chan struct{}

// alternate runs a command with alternating parameters inserted in place of the placeholder. Each
// time a USR1 signal is received, a new command is run with the next parameter, and a TERM signal
// is sent to the previous command after the overlap duration has elapsed. The alternate logs are
// written to stderr, and the command logs are written to cmdStdout and cmdStderr.
func alternate(command, placeholder string, params []string, overlap time.Duration, stderr,
	cmdStdout, cmdStderr io.Writer) {

	setupLog(stderr)

	log.Printf("Starting with command %q, placeholder %q, params = %q, overlap = %v\n",
		command, placeholder, params, overlap)

	// Listen to:
	// - TERM signal (termination signal sent programmatically by e.g. supervisord);
	// - INT signal (termination signal sent when the user presses Ctrl-C in the terminal).
	terminate := make(chan os.Signal)
	signal.Notify(terminate, syscall.SIGTERM, syscall.SIGINT)

	cmdExit := make(chan string)

	overlapEnd := make(chan struct{})

	rotate := make(chan os.Signal)
	signal.Notify(rotate, syscall.SIGUSR1)

	// Convenience closure for easily running a command with a given parameter.
	runFunc := func(param string) (*exec.Cmd, error) {
		log.Printf("Running command with parameter %q\n", param)
		s := strings.Replace(command, placeholder, param, 1)
		c := cmd(s, cmdStdout, cmdStderr)
		return c, runCmd(c, param, cmdExit)
	}

	s := newState(params)

	currentParam, _ := s.current()
	if err := run(s, currentParam, runFunc); err != nil {
		log.Println(err.Error())
		return
	}

	for {
		select {
		case <-testKill:
			log.Println("testKill channel received a value, sending KILL signal to all commands " +
				"and exiting alternate")
			signalAllCmds(s, syscall.SIGKILL)
			return

		case <-terminate:
			log.Println("Received TERM or INT signal, sending TERM signal to all commands, will " +
				"exit after all commands have exited")
			signalAllCmds(s, syscall.SIGTERM)

		case param := <-cmdExit:
			log.Printf("Command with parameter %q exited\n", param)
			s.unset(param)
			if s.empty() {
				log.Println("All commands have exited, exiting alternate")
				return
			}

		case <-overlapEnd:
			finishRotation(s)

		case <-rotate:
			nextParam, _ := s.next()
			log.Printf("Received signal USR1, rotating to next parameter %q", nextParam)

			if err := run(s, nextParam, runFunc); err != nil {
				log.Println(err.Error())
				break
			}

			if overlap == 0 {
				finishRotation(s)
			} else {
				currentParam, _ := s.current()
				log.Printf("Waiting %v before sending TERM signal to command with parameter %q\n",
					overlap, currentParam)
				go countdown(overlap, overlapEnd)
			}
		}
	}
}

func finishRotation(s *state) {
	if _, c := s.next(); c == nil {
		// The next command is not running. Cancel the rotation.
		return
	}
	terminateCurrentCmd(s)
	s.rotate()
}

func run(s *state, param string, runFunc runFunc) error {
	if c := s.cmd(param); c != nil {
		return fmt.Errorf("A command with parameter %q is already running, cannot run again",
			param)
	}

	c, err := runFunc(param)
	if err != nil {
		return fmt.Errorf("Failed to run the command with parameter %q, error: %v\n",
			param, err.Error())
	}

	s.set(param, c)
	return nil
}

func terminateCurrentCmd(s *state) {
	if p, c := s.current(); c != nil {
		log.Printf("Sending TERM signal to command with parameter %q\n", p)
		if err := signalCmd(c, syscall.SIGTERM); err != nil {
			log.Printf("Failed to send TERM signal to command with parameter %q, error: %v\n",
				p, err)
		}
	}
}

func signalAllCmds(s *state, sig os.Signal) {
	s.each(func(p string, c *exec.Cmd) {
		log.Printf("Sending signal to command with parameter %q\n", p)
		signalCmd(c, sig)
	})
}

func signalCmd(c *exec.Cmd, sig os.Signal) error {
	if c == nil {
		return errors.New("signalCmd error: cmd is nil")
	}
	process := c.Process
	if process == nil {
		return errors.New("signalCmd error: cmd.Process is nil")
	}
	process.Signal(sig)
	return nil
}

func countdown(d time.Duration, end chan struct{}) {
	time.Sleep(d)
	end <- struct{}{}
}

// cmd returns a command built from the given string. The command prints to the given stdout and
// stderr.
func cmd(s string, stdout, stderr io.Writer) *exec.Cmd {
	f := strings.Fields(s)
	c := exec.Command(f[0], f[1:]...)
	c.Stdout = stdout
	c.Stderr = stderr
	return c
}

// runCmd runs a command without blocking. After the command exits, runCmd sends msg on the exit
// channel.
func runCmd(c *exec.Cmd, msg string, exit chan string) error {
	if err := c.Start(); err != nil {
		return err
	}
	go func() {
		c.Wait()
		exit <- msg
	}()
	return nil
}

func setupLog(output io.Writer) {
	log.SetPrefix("alternate | ")
	log.SetOutput(output)
	log.SetFlags(0)
}
