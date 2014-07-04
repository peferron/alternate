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

// Channel that receives USR2 to exit alternate. Used for testing only, otherwise it's nil.
var usr2 chan os.Signal

func alternate(a arguments, stderr, cmdStdout, cmdStderr io.Writer) {
	log.SetPrefix("alternate | ")
	log.SetOutput(stderr)
	log.SetFlags(0)

	log.Printf("Starting with command = '%s', values = %v, overlap = %vs\n",
		a.command, a.values, a.overlap.Seconds())

	// Channel that receives SIGUSR1 to execute the next command.
	usr1 := make(chan os.Signal, 1)
	signal.Notify(usr1, syscall.SIGUSR1)
	// Get the loop started.
	usr1 <- syscall.SIGUSR1

	// Channel that receives command exits.
	cmdExit := make(chan string, 1)

	overlap := make(chan struct{}, 1)

	l := len(a.values)
	i := -1

	cmds := map[string]*exec.Cmd{}

	for {
		select {
		case <-usr2:
			log.Println("Received USR2, exiting")
			return

		case <-usr1:
			log.Println("Received USR1")

			// Can the next command be run?
			nextValue := a.values[(i+1)%l]
			if _, ok := cmds[nextValue]; ok {
				log.Printf("Command with value '%s' still running, cannot run again\n", nextValue)
				break
			}
			s := strings.Replace(a.command, "%alt", nextValue, 1)
			nextCmd := command(s, cmdStdout, cmdStderr)
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

			if a.overlap > 0 {
				// If the next command is still running after the overlap, send SIGINT to the
				// current command.
				go func() {
					time.Sleep(a.overlap)
					overlap <- struct{}{}
				}()
			} else {
				// Send SIGINT to the current command right now.
				currentValue := a.values[i%l]
				if currentCmd, ok := cmds[currentValue]; ok {
					if p := currentCmd.Process; p != nil {
						log.Printf("Sending immediate SIGINT to command with value '%s'",
							currentValue)
						p.Signal(os.Interrupt)
						i++
					}
				}
			}

		case <-overlap:
			nextValue := a.values[(i+1)%l]
			if _, ok := cmds[nextValue]; ok {
				currentValue := a.values[i%l]
				if currentCmd, ok := cmds[currentValue]; ok {
					if p := currentCmd.Process; p != nil {
						log.Printf("Overlap finished, sending SIGINT to command with value '%s'",
							currentValue)
						p.Signal(os.Interrupt)
						i++
					}
				}
			}

		case v := <-cmdExit:
			log.Printf("Command with value '%s' exited\n", v)
			delete(cmds, v)
			if len(cmds) == 0 {
				log.Println("No command running anymore, exiting")
				return
			}
		}
	}
}

func command(s string, stdout, stderr io.Writer) *exec.Cmd {
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
