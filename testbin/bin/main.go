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
	exitAfterSigtermDelay, err := time.ParseDuration(b[1])
	if err != nil {
		panic(err)
	}

	p := fmt.Sprintf("%s %s | ", rawArgs, rawBehavior)
	log.SetPrefix(p)
	log.SetFlags(0)

	logf("start\n")

	exit := make(chan struct{})
	go exitAfterStart(exitAfterStartDelay, exit)
	go exitAfterSigterm(exitAfterSigtermDelay, exit)

	<-exit
	logf("exit")
}

func exitAfterStart(delay time.Duration, exit chan struct{}) {
	if delay >= 0 {
		time.Sleep(delay)
		exit <- struct{}{}
	}
}

func exitAfterSigterm(delay time.Duration, exit chan struct{}) {
	term := make(chan os.Signal)
	signal.Notify(term, syscall.SIGTERM, syscall.SIGINT)
	for _ = range term {
		if delay >= 0 {
			time.Sleep(delay)
			close(term)
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
