package main

import (
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
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
	cmdExit := make(chan int, 1)

	l := len(a.values)
	i := 0
	cmds := make([]*exec.Cmd, l)

	for {
		select {
		case <-usr2:
			log.Println("Received USR2")
			return
		case <-usr1:
			log.Println("Received USR1")
			if cmds[i] != nil {
				log.Printf("Command %d still running, cannot run again\n", i)
				break
			}
			c := command(a, i, cmdStdout, cmdStderr)
			cmds[i] = c
			go run(c, cmdExit, i)
			i = rotate(i, l)
		case j := <-cmdExit:
			log.Printf("Command #%d exited\n", j)
			cmds[j] = nil
		}
	}
}

func command(a arguments, i int, stdout, stderr io.Writer) *exec.Cmd {
	s := strings.Replace(a.command, "%alternate", a.values[i], 1)

	c := exec.Command("sh", "-c", s)
	c.Stdout = stdout
	c.Stderr = stderr

	return c
}

func run(c *exec.Cmd, exit chan int, i int) {
	log.Printf("Running command #%d\n", i)
	c.Run()
	exit <- i
}

func rotate(i, l int) int {
	if i < l-1 {
		return i + 1
	}
	return 0
}
