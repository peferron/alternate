package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/peferron/alternate/testbin"
)

type arguments struct {
	arbitraryArg    string
	behavior        string
	exitAfterStart  time.Duration
	exitAfterSigint time.Duration
}

func main() {
	rawArgs := strings.Join(os.Args[1:], " ")
	rawBehavior := testbin.GetBehavior()

	b := strings.Fields(rawBehavior)
	exitAfterStartDelay, err := time.ParseDuration(b[0])
	if err != nil {
		panic(err)
	}
	exitAfterSigintDelay, err := time.ParseDuration(b[1])
	if err != nil {
		panic(err)
	}

	p := fmt.Sprintf("%s %s | ", rawArgs, rawBehavior)
	log.SetPrefix(p)
	log.SetFlags(0)

	logf("start\n")

	exit := make(chan struct{})
	go exitAfterStart(exitAfterStartDelay, exit)
	go exitAfterSigint(exitAfterSigintDelay, exit)

	<-exit
	logf("exit")
}

func exitAfterStart(delay time.Duration, exit chan struct{}) {
	if delay >= 0 {
		time.Sleep(delay)
		exit <- struct{}{}
	}
}

func exitAfterSigint(delay time.Duration, exit chan struct{}) {
	sigint := make(chan os.Signal)
	signal.Notify(sigint, syscall.SIGINT)
	for _ = range sigint {
		if delay >= 0 {
			time.Sleep(delay)
			close(sigint)
		}
	}
	exit <- struct{}{}
}

func logf(format string, a ...interface{}) {
	log.SetOutput(os.Stdout)
	log.Printf(format, a...)
	log.SetOutput(os.Stderr)
	log.Printf(format, a...)
}
