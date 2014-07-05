package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/peferron/alternate/testbin"
)

const (
	zero  time.Duration = 0
	one                 = 10 * time.Millisecond
	two                 = 2 * one
	three               = 3 * one
)

func init() {
	fmt.Println("Initializing testExit channel")
	testExit = make(chan struct{})
}

func newNilWriter() *nilWriter {
	return &nilWriter{}
}

type nilWriter struct{}

func (w *nilWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}

func newLineWriter(log bool) *lineWriter {
	return &lineWriter{
		"",
		[]string{},
		&sync.Mutex{},
		log,
	}
}

type lineWriter struct {
	buf   string
	lines []string
	mutex *sync.Mutex
	log   bool
}

func (w *lineWriter) Write(p []byte) (n int, err error) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	n = len(p)

	w.buf += string(p)
	a := strings.Split(w.buf, "\n")
	for _, s := range a {
		if s == "" {
			continue
		}
		w.lines = append(w.lines, s)
		if w.log {
			fmt.Printf("LineWriter received: %q\n", s)
		}
	}
	w.buf = a[len(a)-1]

	return
}

func (w *lineWriter) reset() {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	w.buf = ""
	w.lines = []string{}
}

func (w *lineWriter) getLines() []string {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	clone := make([]string, len(w.lines))
	for i, s := range w.lines {
		clone[i] = s
	}
	return clone
}

type test struct {
	t           *testing.T
	cmdStdout   *lineWriter
	cmdStderr   *lineWriter
	params      []string
	expectIndex int
}

func (test *test) reset() {
	test.cmdStdout.reset()
	test.cmdStderr.reset()
}

func (test *test) expect(d time.Duration, lines []string) {
	expectIndex := test.expectIndex
	test.expectIndex++

	time.Sleep(d)

	stdoutLines := test.cmdStdout.getLines()
	if !sameStrings(lines, stdoutLines) {
		fmt.Printf("For parameters %q expect #%d, within %v expected lines %q in stdout, was %q\n",
			test.params, expectIndex, d, lines, stdoutLines)
		test.t.Errorf("For parameters %q expect #%d, within %v expected lines %q in stdout, was %q",
			test.params, expectIndex, d, lines, stdoutLines)
	}
	stderrLines := test.cmdStderr.getLines()
	if !sameStrings(lines, stderrLines) {
		fmt.Printf("For parameters %q expect #%d, within %v expected lines %q in stderr, was %q\n",
			test.params, expectIndex, d, lines, stderrLines)
		test.t.Errorf("For parameters %q expect #%d, within %v expected lines %q in stderr, was %q",
			test.params, expectIndex, d, lines, stderrLines)
	}
}

// sameStrings makes an unordered comparison of two slices of strings, and returns true if they
// contain the same strings, or false otherwise.
func sameStrings(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	if len(a) == 0 {
		return true
	}
	aa := clone(a)
	bb := clone(b)
	sort.Strings(aa)
	sort.Strings(bb)
	for i := range aa {
		if aa[i] != bb[i] {
			return false
		}
	}
	return true
}

// clone returns a deep copy of a slice of strings.
func clone(a []string) []string {
	b := make([]string, len(a))
	for i := range a {
		b[i] = a[i]
	}
	return b
}

func start(t *testing.T, params []string, overlap time.Duration) *test {
	test := &test{
		t,
		newLineWriter(false),
		newLineWriter(false),
		params,
		0,
	}
	c := testbin.Build() + " " + placeholder
	go alternate(c, placeholder, params, overlap, newNilWriter(), test.cmdStdout, test.cmdStderr)
	return test
}

func sendUsr1() {
	process().Signal(syscall.SIGUSR1)
}

func stop() {
	testExit <- struct{}{}
	time.Sleep(one)
}

func process() *os.Process {
	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		panic(err)
	}
	return p
}

func TestLineWriter(t *testing.T) {
	type action struct {
		reset bool
		write string
	}
	tests := []struct {
		actions []action
		lines   []string
	}{
		{
			[]action{
				{false, "abc\nde\n\nf"},
			},
			[]string{"abc", "de", "f"},
		},
	}

	for i, test := range tests {
		writer := newLineWriter(false)
		for _, a := range test.actions {
			if a.reset {
				writer.reset()
			}
			if a.write != "" {
				writer.Write([]byte(a.write))
			}
		}

		lines := writer.getLines()

		same := true
		if len(test.lines) != len(lines) {
			same = false
		} else {
			for j := range test.lines {
				if test.lines[j] != lines[j] {
					same = false
					break
				}
			}
		}

		if !same {
			t.Errorf("For test #%d, expected lines to be %q, but was %q", i, test.lines, lines)
		}
	}
}

func TestNoOverlapNoConflict(t *testing.T) {
	paramSets := [][]string{
		{"param0", "param1"},
		{"param0", "param1", "param0"},
	}
	o := zero

	for _, v := range paramSets {
		a := testbin.SetBehavior(-one, zero, "a")
		test := start(t, v, o)
		test.expect(one, []string{
			"param0 " + a + " | start",
		})

		b := testbin.SetBehavior(-one, zero, "b")
		test.reset()
		sendUsr1()
		test.expect(one, []string{
			"param1 " + b + " | start",
			"param0 " + a + " | exit",
		})

		c := testbin.SetBehavior(-one, zero, "c")
		test.reset()
		sendUsr1()
		test.expect(one, []string{
			"param0 " + c + " | start",
			"param1 " + b + " | exit",
		})

		stop()
	}
}

func TestOverlapNoConflict(t *testing.T) {
	paramSets := [][]string{
		{"param0", "param1"},
		{"param0", "param1", "param0"},
	}
	o := two

	for _, v := range paramSets {
		a := testbin.SetBehavior(-one, zero, "a")
		test := start(t, v, o)
		test.expect(one, []string{
			"param0 " + a + " | start",
		})

		b := testbin.SetBehavior(-one, zero, "b")
		test.reset()
		sendUsr1()
		test.expect(one, []string{
			"param1 " + b + " | start",
		})
		test.reset()
		test.expect(two, []string{
			"param0 " + a + " | exit",
		})

		c := testbin.SetBehavior(-one, zero, "c")
		test.reset()
		sendUsr1()
		test.expect(one, []string{
			"param0 " + c + " | start",
		})
		test.reset()
		test.expect(two, []string{
			"param1 " + b + " | exit",
		})

		stop()
	}
}

func TestNoOverlapConflict(t *testing.T) {
	paramSets := [][]string{
		{"param0", "param1"},
		{"param0", "param1", "param0"},
	}
	o := zero

	for _, v := range paramSets {
		a := testbin.SetBehavior(-one, -one, "a")
		test := start(t, v, o)
		test.expect(one, []string{
			"param0 " + a + " | start",
		})

		b := testbin.SetBehavior(-one, zero, "b")
		test.reset()
		sendUsr1()
		test.expect(one, []string{
			"param1 " + b + " | start",
		})

		testbin.SetBehavior(-one, zero, "c")
		test.reset()
		sendUsr1()
		test.expect(one, []string{})

		stop()
	}
}

func TestOverlapConflict(t *testing.T) {
	paramSets := [][]string{
		{"param0", "param1"},
		{"param0", "param1", "param0"},
	}
	o := two

	for _, v := range paramSets {
		a := testbin.SetBehavior(-one, -one, "a")
		test := start(t, v, o)
		test.expect(one, []string{
			"param0 " + a + " | start",
		})

		b := testbin.SetBehavior(-one, zero, "b")
		test.reset()
		sendUsr1()
		test.expect(one, []string{
			"param1 " + b + " | start",
		})
		test.reset()
		test.expect(two, []string{})

		testbin.SetBehavior(-one, zero, "c")
		test.reset()
		sendUsr1()
		test.expect(three, []string{})

		stop()
	}
}

func TestPrematureCmdExit(t *testing.T) {
	paramSets := [][]string{
		{"param0", "param1"},
		{"param0", "param1", "param0"},
	}
	o := two

	for _, v := range paramSets {
		a := testbin.SetBehavior(-one, zero, "a")
		test := start(t, v, o)
		test.expect(one, []string{
			"param0 " + a + " | start",
		})

		b := testbin.SetBehavior(zero, zero, "b")
		test.reset()
		sendUsr1()
		test.expect(one, []string{
			"param1 " + b + " | start",
			"param1 " + b + " | exit",
		})
		test.reset()
		test.expect(two, []string{})

		c := testbin.SetBehavior(-one, zero, "c")
		test.reset()
		sendUsr1()
		test.expect(one, []string{
			"param1 " + c + " | start",
		})
		test.reset()
		test.expect(two, []string{
			"param0 " + a + " | exit",
		})

		stop()
	}
}

func TestCmdRunError(t *testing.T) {
	paramSets := [][]string{
		{"param0", "param1"},
		{"param0", "param1", "param0"},
	}
	o := zero

	for _, v := range paramSets {
		a := testbin.SetBehavior(-one, zero, "a")
		test := start(t, v, o)
		test.expect(one, []string{
			"param0 " + a + " | start",
		})

		b := testbin.SetBehavior(-one, zero, "b")
		test.reset()
		testbin.Clean()
		sendUsr1()
		test.expect(one, []string{})

		test.reset()
		sendUsr1()
		test.expect(one, []string{})

		testbin.Build()
		test.reset()
		sendUsr1()
		test.expect(one, []string{
			"param1 " + b + " | start",
			"param0 " + a + " | exit",
		})

		stop()
	}
}

func TestAlternateExit(t *testing.T) {
	c := testbin.Build() + " " + placeholder
	v := []string{"param0"}
	o := zero

	testbin.SetBehavior(two, zero, "a")

	exited := false
	go func() {
		alternate(c, placeholder, v, o, newNilWriter(), newNilWriter(), newNilWriter())
		exited = true
	}()

	time.Sleep(one)
	if exited {
		t.Error("Was expecting exited to be false, was true")
	}

	time.Sleep(two)
	if !exited {
		t.Error("Was expecting exited to be true, was false")
	}
}
