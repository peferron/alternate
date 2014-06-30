package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/peferron/alternate/testbin"
)

var savedTestbinPath = ""

func init() {
	usr2 = make(chan os.Signal)
	signal.Notify(usr2, syscall.SIGUSR2)
}

func newNilWriter() *nilWriter {
	return &nilWriter{}
}

type nilWriter struct{}

func (w *nilWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}

func newSearchWriter(s string) *searchWriter {
	return &searchWriter{strings.TrimRight(s, "\n"), "", false}
}

type searchWriter struct {
	search string
	buf    string
	found  bool
}

func (w *searchWriter) Write(p []byte) (n int, err error) {
	n = len(p)

	if w.found {
		return
	}

	w.buf += string(p)
	a := strings.Split(w.buf, "\n")
	for _, v := range a {
		if v == w.search {
			w.found = true
			return
		}
	}
	w.buf = a[len(a)-1]

	return
}

func (w *searchWriter) reset(s string) {
	w.search = strings.TrimRight(s, "\n")
	w.buf = ""
	w.found = false
}

func TestSearchWriter(t *testing.T) {
	type op struct {
		reset string
		write string
		found bool
	}

	tests := []struct {
		search string
		ops    []op
	}{
		{"hit", []op{
			{"", "miss\n", false},
		}},
		{"hit", []op{
			{"", "hit\n", true},
		}},
		{"hit\n", []op{
			{"", "hit\n", true},
		}},
		{"hit", []op{
			{"", "miss\nhit\n", true},
		}},
		{"hit", []op{
			{"", "hit\nmiss\n", true},
		}},
		{"hit", []op{
			{"", "miss\nhit\nmiss\n", true},
		}},
		{"hit", []op{
			{"", "hit\n", true},
			{"", "miss\n", true},
			{"hit2", "", false},
			{"", "hit\n", false},
			{"", "hit2\n", true},
		}},
	}

	for i, test := range tests {
		writer := newSearchWriter(test.search)
		for j, o := range test.ops {
			if o.reset != "" {
				writer.reset(o.reset)
				continue
			}

			writer.Write([]byte(o.write))
			found := writer.found
			if o.found != found {
				t.Errorf("In test %d op %d, expected found to be %t, was %t",
					i, j, o.found, found)
			}
		}
	}
}

func TestNoConflict(t *testing.T) {
	bin := testbinPath()

	// To free the slot immediately, return immediately after start and use zero overlap.
	c := makeTestCommand(bin, 0, -1*time.Second)
	v := []string{"val0", "val1"}
	a := arguments{c, v, 0}

	// Check first exec (first value).
	stderr := newNilWriter()
	cmdStdout := newSearchWriter("testbin[val0] | Print to stdout")
	cmdStderr := newSearchWriter("testbin[val0] | Print to stderr")
	startAlternate(a, stderr, cmdStdout, cmdStderr)
	time.Sleep(100 * time.Millisecond)
	if !cmdStdout.found {
		t.Error("Expected cmdStdout.found to be true (1st exec), was false")
	}
	if !cmdStderr.found {
		t.Error("Expected cmdStderr.found to be true (1st exec), was false")
	}

	// Check second exec (second value).
	cmdStdout.reset("testbin[val1] | Print to stdout")
	cmdStderr.reset("testbin[val1] | Print to stderr")
	triggerAlternate()
	if !cmdStdout.found {
		t.Error("Expected cmdStdout.found to be true (2nd exec), was false")
	}
	if !cmdStderr.found {
		t.Error("Expected cmdStderr.found to be true (2nd exec), was false")
	}

	// Check third exec (back to first value). Should not conflict with already-exited first exec.
	cmdStdout.reset("testbin[val0] | Print to stdout")
	cmdStderr.reset("testbin[val0] | Print to stderr")
	triggerAlternate()
	if !cmdStdout.found {
		t.Error("Expected cmdStdout.found to be true (3rd exec), was false")
	}
	if !cmdStderr.found {
		t.Error("Expected cmdStderr.found to be true (3rd exec), was false")
	}

	exitAlternate()
}

func TestConflict(t *testing.T) {
	bin := testbinPath()

	// To keep the slot busy, return 5s after start.
	c := makeTestCommand(bin, 5*time.Second, -1*time.Second)
	v := []string{"val0", "val1"}
	a := arguments{c, v, 0}

	// Check first exec (first value).
	stderr := newNilWriter()
	cmdStdout := newSearchWriter("testbin[val0] | Print to stdout")
	cmdStderr := newSearchWriter("testbin[val0] | Print to stderr")
	startAlternate(a, stderr, cmdStdout, cmdStderr)
	if !cmdStdout.found {
		t.Error("Expected cmdStdout.found to be true (1st exec), was false")
	}
	if !cmdStderr.found {
		t.Error("Expected cmdStderr.found to be true (1st exec), was false")
	}

	// Check second exec (second value).
	cmdStdout.reset("testbin[val1] | Print to stdout")
	cmdStderr.reset("testbin[val1] | Print to stderr")
	triggerAlternate()
	if !cmdStdout.found {
		t.Error("Expected cmdStdout.found to be true (2nd exec), was false")
	}
	if !cmdStderr.found {
		t.Error("Expected cmdStderr.found to be true (2nd exec), was false")
	}

	// Check third exec (back to first value). Should conflict with still-running first exec.
	cmdStdout.reset("testbin[val0] | Print to stdout")
	cmdStderr.reset("testbin[val0] | Print to stderr")
	triggerAlternate()
	if cmdStdout.found {
		t.Error("Expected cmdStdout.found to be false (3rd exec), was true")
	}
	if cmdStderr.found {
		t.Error("Expected cmdStderr.found to be false (3rd exec), was true")
	}

	exitAlternate()
}

func testbinPath() string {
	if savedTestbinPath != "" {
		return savedTestbinPath
	}

	dir := os.TempDir()
	path := path.Join(dir, "testbin")
	pkg := testbin.BinPkgPath()

	c := exec.Command("go", "build", "-o", path, pkg)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	fmt.Printf("Building testbin: %v\n", c.Args)

	if err := c.Run(); err != nil {
		panic(err)
	}

	savedTestbinPath = path
	return path
}

func makeTestCommand(testbin string, exitAfterStart, exitAfterSigint time.Duration) string {
	return fmt.Sprintf("%s %%alternate %s %s", testbin, exitAfterStart, exitAfterSigint)
}

func startAlternate(a arguments, stderr, cmdStdout, cmdStderr io.Writer) {
	go alternate(a, stderr, cmdStdout, cmdStderr)
	time.Sleep(100 * time.Millisecond)
}

func triggerAlternate() {
	syscall.Kill(syscall.Getpid(), syscall.SIGUSR1)
	time.Sleep(100 * time.Millisecond)
}

func exitAlternate() {
	syscall.Kill(syscall.Getpid(), syscall.SIGUSR2)
	time.Sleep(100 * time.Millisecond)
}
