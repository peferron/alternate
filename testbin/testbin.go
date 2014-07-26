package testbin

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"reflect"
	"time"
)

type t struct{}

var (
	built     = false
	buildPath = ""
)

const behaviorKey = "ALTERNATE_TESTBIN_BEHAVIOR"

func GetBehavior() string {
	return os.Getenv(behaviorKey)
}
func SetBehavior(exitAfterStartDelay, exitAfterSigintDelay time.Duration, label string) string {
	s := fmt.Sprintf("%v %v %s", exitAfterStartDelay, exitAfterSigintDelay, label)
	os.Setenv(behaviorKey, s)
	return s
}

func Build() string {
	if built {
		return buildPath
	}

	if buildPath == "" {
		dir, err := ioutil.TempDir("", "testbin_")
		if err != nil {
			panic(err)
		}
		p := path.Join(dir, "testbin")
		build(p)
		buildPath = p
	} else {
		build(buildPath)
	}

	built = true
	return buildPath
}

func Clean() {
	if !built {
		return
	}
	if err := os.Remove(buildPath); err == nil {
		built = false
	}
}

func binPkgPath() string {
	return reflect.TypeOf(t{}).PkgPath() + "/bin"
}

func build(p string) {
	c := exec.Command("go", "build", "-o", p, binPkgPath())
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	fmt.Printf("Building testbin: %v\n", c.Args)

	if err := c.Run(); err != nil {
		panic(err)
	}
}
