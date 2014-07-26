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
	one                 = 50 * time.Millisecond
	two                 = 2 * one
	three               = 3 * one
	four                = 4 * one
	five                = 5 * one
)

func init() {
	fmt.Println("Initializing testKill channel")
	testKill = make(chan struct{})
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
	return clone(w.lines)
}

type test struct {
	t           *testing.T
	cmdStdout   *lineWriter
	cmdStderr   *lineWriter
	params      []string
	exited      bool
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
	copy(b, a)
	return b
}

func newTest(t *testing.T, params []string, overlap time.Duration) *test {
	command := testbin.Build() + " " + placeholder
	return newTestWithCommand(t, params, overlap, command)
}

func newTestWithCommand(t *testing.T, params []string, overlap time.Duration,
	command string) *test {
	test := &test{
		t,
		newLineWriter(false),
		newLineWriter(false),
		params,
		false,
		0,
	}
	go func() {
		alternate(command, placeholder, params, overlap, newNilWriter(), test.cmdStdout,
			test.cmdStderr)
		test.exited = true
	}()
	return test
}

func sendUsr1() {
	process().Signal(syscall.SIGUSR1)
}

func sendTerm() {
	process().Signal(syscall.SIGTERM)
}

func kill() {
	testKill <- struct{}{}
	time.Sleep(one)
}

func process() *os.Process {
	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		panic(err)
	}
	return p
}

func TestSameStrings(t *testing.T) {
	tests := []struct {
		a    []string
		b    []string
		same bool
	}{
		{[]string{"a", "b"}, []string{"a", "b"}, true},
		{[]string{"b", "a"}, []string{"a", "b"}, true},
		{[]string{"a", "b", "c"}, []string{"a", "b"}, false},
	}

	for _, test := range tests {
		same := sameStrings(test.a, test.b)
		if test.same != same {
			t.Errorf("For values %q and %q, expected same to be %t, was %t",
				test.a, test.b, test.same, same)
		}
	}
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
	paramsList := [][]string{
		{"param0", "param1"},
		{"param0", "param1", "param0"},
	}
	overlap := zero

	for _, params := range paramsList {
		a := testbin.SetBehavior(-one, zero, "a")
		test := newTest(t, params, overlap)
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

		kill()
	}
}

func TestOverlapNoConflict(t *testing.T) {
	paramsList := [][]string{
		{"param0", "param1"},
		{"param0", "param1", "param0"},
	}
	overlap := two

	for _, params := range paramsList {
		a := testbin.SetBehavior(-one, zero, "a")
		test := newTest(t, params, overlap)
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

		kill()
	}
}

func TestNoOverlapConflict(t *testing.T) {
	paramsList := [][]string{
		{"param0", "param1"},
		{"param0", "param1", "param0"},
	}
	overlap := zero

	for _, params := range paramsList {
		a := testbin.SetBehavior(-one, -one, "a")
		test := newTest(t, params, overlap)
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

		kill()
	}
}

func TestOverlapConflict(t *testing.T) {
	paramsList := [][]string{
		{"param0", "param1"},
		{"param0", "param1", "param0"},
	}
	overlap := two

	for _, params := range paramsList {
		a := testbin.SetBehavior(-one, -one, "a")
		test := newTest(t, params, overlap)
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

		kill()
	}
}

func TestPrematureCurrentCmdExit(t *testing.T) {
	paramsList := [][]string{
		{"param0", "param1", "param2"},
	}
	overlap := five

	for _, params := range paramsList {
		a := testbin.SetBehavior(three, zero, "a")
		test := newTest(t, params, overlap)
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

		testbin.SetBehavior(-one, zero, "c")
		test.reset()
		sendUsr1()
		test.expect(three, []string{})

		d := testbin.SetBehavior(-one, zero, "d")
		test.reset()
		sendUsr1()
		test.expect(one, []string{
			"param2 " + d + " | start",
		})

		kill()
	}
}

func TestPrematureNextCmdExit(t *testing.T) {
	paramsList := [][]string{
		{"param0", "param1"},
		{"param0", "param1", "param0"},
	}
	overlap := two

	for _, params := range paramsList {
		a := testbin.SetBehavior(-one, zero, "a")
		test := newTest(t, params, overlap)
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

		kill()
	}
}

func TestCmdRunError(t *testing.T) {
	paramsList := [][]string{
		{"param0", "param1"},
		{"param0", "param1", "param0"},
	}
	overlap := zero

	for _, params := range paramsList {
		a := testbin.SetBehavior(-one, zero, "a")
		test := newTest(t, params, overlap)
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

		kill()
	}
}

func TestFirstCmdRunError(t *testing.T) {
	params := []string{"param0"}
	overlap := zero

	command := testbin.Build() + "_fake " + placeholder
	test := newTestWithCommand(t, params, overlap, command)
	test.expect(one, []string{})
	if !test.exited {
		t.Error("Was expecting exited to be true, was false")
	}
}

func TestAllCmdsExit(t *testing.T) {
	params := []string{"param0"}
	overlap := zero

	a := testbin.SetBehavior(two, zero, "a")
	test := newTest(t, params, overlap)
	test.expect(one, []string{
		"param0 " + a + " | start",
	})
	if test.exited {
		t.Error("Was expecting exited to be false, was true")
	}

	test.reset()
	test.expect(two, []string{
		"param0 " + a + " | exit",
	})
	if !test.exited {
		t.Error("Was expecting exited to be true, was false")
	}
}

func TestTermForwarding(t *testing.T) {
	params := []string{"param0"}
	overlap := zero

	a := testbin.SetBehavior(-one, two, "a")
	test := newTest(t, params, overlap)
	test.expect(one, []string{
		"param0 " + a + " | start",
	})
	if test.exited {
		t.Error("Was expecting exited to be false, was true")
	}

	test.reset()
	sendTerm()
	test.expect(one, []string{})
	if test.exited {
		t.Error("Was expecting exited to be false, was true")
	}

	test.reset()
	test.expect(two, []string{
		"param0 " + a + " | exit",
	})
	if !test.exited {
		t.Error("Was expecting exited to be true, was false")
	}
}
