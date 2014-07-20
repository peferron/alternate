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

// testKill is a channel that is used during testing only to trigger an immediate cleanup and return
// from the alternate function.
var testKill chan struct{}

// alternate runs a command with alternating parameters inserted in place of the placeholder. Each
// time a USR1 signal is received, a new command is run with the next parameter, and a TERM signal
// is sent to the previous command after the overlap duration has elapsed. The alternate logs are
// written to stderr, and the command logs are written to cmdStdout and cmdStderr.
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

	term := make(chan os.Signal)
	// Listen to both TERM signal (termination signal sent programmatically by e.g. supervisord)
	// and INT signal (termination signal sent when the user presses Ctrl-C in the terminal).
	signal.Notify(term, syscall.SIGTERM, syscall.SIGINT)

	s := newState(params)

	for {
		select {
		case <-testKill:
			log.Println("testKill channel received a value, sending KILL signal to all commands " +
				"and exiting alternate")
			signalAllCmds(params, s, syscall.SIGKILL)
			return

		case <-term:
			log.Println("Received TERM or INT signal, sending TERM signal to all commands, will " +
				"exit after all commands have exited")
			signalAllCmds(params, s, syscall.SIGTERM)

		case param := <-cmdExit:
			log.Printf("Command with parameter %q exited\n", param)
			s.unsetCmd(param)
			if !s.hasCmds() {
				log.Println("All commands have exited, exiting alternate")
				return
			}

		case <-overlapEnd:
			finishRotation(s)

		case sig := <-next:
			first := sig != syscall.SIGUSR1
			nextParam := s.nextParam()

			if !first {
				log.Printf("Received signal USR1, rotating to next parameter %q", nextParam)
			}

			if s.nextCmd() != nil {
				log.Printf("A command with parameter %q is already running, cannot run again",
					nextParam)
				break
			}

			cmdStr := strings.Replace(command, placeholder, nextParam, 1)
			nextCmd := cmd(cmdStr, cmdStdout, cmdStderr)
			s.setCmd(nextParam, nextCmd)

			log.Printf("Running command with parameter %q\n", nextParam)
			if err := run(nextCmd, nextParam, cmdExit); err != nil {
				log.Printf("Failed to run the command with parameter %q, error: %v\n",
					err.Error())
				s.unsetCmd(nextParam)
				break
			}

			if first {
				// There is no previous command to terminate.
				s.rotate()
				break
			}

			if overlap == 0 {
				finishRotation(s)
			} else {
				log.Printf("Waiting %v before sending TERM signal to command with parameter %q\n",
					overlap, s.currentParam())
				go func() {
					time.Sleep(overlap)
					overlapEnd <- struct{}{}
				}()
			}
		}
	}
}

func finishRotation(s *state) {
	if s.nextCmd() == nil {
		// The next command is not running. Cancel the rotation without terminating the current
		// command.
		return
	}
	if c := s.currentCmd(); c != nil {
		currentParam := s.currentParam()
		log.Printf("Sending TERM signal to command with parameter %q\n", currentParam)
		if err := signalCmd(c, syscall.SIGTERM); err != nil {
			log.Printf("Failed to send TERM signal to command with parameter %q, error: %v\n",
				currentParam, err)
		}
	}
	s.rotate()
}

// signalCmd sends a signal to a command.
func signalCmd(c *exec.Cmd, s os.Signal) error {
	if c == nil {
		return errors.New("signalCmd error: cmd is nil")
	}
	p := c.Process
	if p == nil {
		return errors.New("signalCmd error: cmd.Process is nil")
	}
	p.Signal(s)
	return nil
}

// signalAllCmds sends a signal to all commands in the state.
func signalAllCmds(params []string, s *state, sig os.Signal) {
	for _, param := range params {
		if c := s.cmd(param); c != nil {
			log.Printf("Sending signal to command with parameter %q\n", param)
			signalCmd(c, sig)
		}
	}
}

// cmd returns a command built from the given string. The command will print to the given stdout and
// stderr.
func cmd(s string, stdout, stderr io.Writer) *exec.Cmd {
	f := strings.Fields(s)
	c := exec.Command(f[0], f[1:]...)
	c.Stdout = stdout
	c.Stderr = stderr
	return c
}

// run runs a command without blocking. After the command exits, it sends exitMsg on the exit
// channel.
func run(c *exec.Cmd, exitMsg string, exit chan string) error {
	if err := c.Start(); err != nil {
		return err
	}
	go func() {
		c.Wait()
		exit <- exitMsg
	}()
	return nil
}
