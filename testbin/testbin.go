package testbin

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"reflect"
	"time"
)

type t struct{}

var savedBinPath = ""

const behaviorKey = "ALTERNATE_TESTBIN_BEHAVIOR"

func GetBehavior() string {
	return os.Getenv(behaviorKey)
}
func SetBehavior(exitAfterStartDelay, exitAfterSigintDelay time.Duration, label string) string {
	s := fmt.Sprintf("%v %v %s", exitAfterStartDelay, exitAfterSigintDelay, label)
	os.Setenv(behaviorKey, s)
	return s
}

func Path() string {
	if savedBinPath != "" {
		return savedBinPath
	}

	dir := os.TempDir()
	path := path.Join(dir, "testbin")

	c := exec.Command("go", "build", "-o", path, binPkgPath())
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	fmt.Printf("Building testbin: %v\n", c.Args)

	if err := c.Run(); err != nil {
		panic(err)
	}

	savedBinPath = path
	return path
}

func binPkgPath() string {
	return reflect.TypeOf(t{}).PkgPath() + "/bin"
}
