package testbin

import "reflect"

type t struct{}

func BinPkgPath() string {
	return reflect.TypeOf(t{}).PkgPath() + "/bin"
}
