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

// exit is a channel that is used during testing only, to return from the alternate function.
var exit chan struct{}

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

	// cmds maps values to their running commands.
	cmds := map[string]*exec.Cmd{}

	// i is the index of the currently running command. It increases by 1 every time alternate
	// successfully moves to the next value.
	i := -1

	for {
		select {
		case <-exit:
			log.Println("Exit channel triggered, exiting")
			return

		case value := <-cmdExit:
			log.Printf("Command with value '%s' exited\n", value)
			delete(cmds, value)
			if len(cmds) == 0 {
				log.Println("No running commands, exiting")
				return
			}

		case signal := <-next:
			l := len(values)
			nextValue := values[(i+1)%l]

			if signal == syscall.SIGUSR1 {
				log.Printf("Received USR1, trying to move to next value: '%s'", nextValue)
			}

			if _, ok := cmds[nextValue]; ok {
				log.Printf("Command with value '%s' still running, cannot run again\n", nextValue)
				break
			}

			s := strings.Replace(command, placeholder, nextValue, 1)
			nextCmd := cmd(s, cmdStdout, cmdStderr)
			cmds[nextValue] = nextCmd
			if err := run(nextCmd, cmdExit, nextValue); err != nil {
				log.Printf("Command with value '%s' failed to run, error: '%s'\n", err.Error())
				delete(cmds, nextValue)
				break
			}

			if i < 0 {
				i++
				break
			}

			if overlap == 0 {
				if interruptCurrentCmd(cmds, values, i) {
					i++
				}
			} else {
				go func() {
					time.Sleep(overlap)
					overlapEnd <- struct{}{}
				}()
			}

		case <-overlapEnd:
			if interruptCurrentCmd(cmds, values, i) {
				i++
			}
		}
	}
}

func interruptCurrentCmd(cmds map[string]*exec.Cmd, values []string, i int) bool {
	l := len(values)
	nextValue := values[(i+1)%l]
	if _, ok := cmds[nextValue]; !ok {
		// The next command exited prematurely
		return false
	}

	currentValue := values[i%l]
	currentCmd, ok := cmds[currentValue]
	if !ok {
		return false
	}

	p := currentCmd.Process
	if p == nil {
		return false
	}

	log.Printf("Overlap finished, sending SIGINT to command with value '%s'",
		currentValue)
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
