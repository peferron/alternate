package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type arguments struct {
	arbitraryArg    string
	exitAfterStart  time.Duration
	exitAfterSigint time.Duration
}

func main() {
	a := args()

	p := fmt.Sprintf("testbin[%s] | ", a.arbitraryArg)
	log.SetPrefix(p)
	log.SetFlags(0)

	log.Printf("Arguments: arbitraryArg = '%s', exitAfterStart = %vs, exitAfterSigint = %vs\n",
		a.arbitraryArg, a.exitAfterStart.Seconds(), a.exitAfterSigint.Seconds())

	// Test printing to stdout and stderr.
	log.SetOutput(os.Stdout)
	log.Println("Print to stdout")
	log.SetOutput(os.Stderr)
	log.Println("Print to stderr")

	if a.exitAfterStart >= 0 {
		go exitAfterStart(a.exitAfterStart)
	}

	if a.exitAfterSigint >= 0 {
		go exitAfterSigint(a.exitAfterSigint)
	}

	time.Sleep(time.Hour)
}

func args() arguments {
	arbitraryArg := os.Args[1]

	exitAfterStart, err := time.ParseDuration(os.Args[2])
	if err != nil {
		panic(err)
	}

	exitAfterSigint, err := time.ParseDuration(os.Args[3])
	if err != nil {
		panic(err)
	}

	return arguments{
		arbitraryArg,
		exitAfterStart,
		exitAfterSigint,
	}
}

func exitAfterStart(delay time.Duration) {
	log.Printf("exitAfterStart: Will exit after %vs\n", delay.Seconds())
	time.Sleep(delay)
	log.Println("exitAfterStart: Exiting now")
	os.Exit(0)
}

func exitAfterSigint(delay time.Duration) {
	log.Println("exitAfterSigint: Waiting for SIGINT")
	sigint := make(chan os.Signal)
	signal.Notify(sigint, syscall.SIGINT)
	<-sigint

	log.Printf("exitAfterSigint: Received SIGINT, will exit after %vs\n", delay.Seconds())
	time.Sleep(delay)

	log.Println("exitAfterSigint: Exiting now")
	os.Exit(0)
}
