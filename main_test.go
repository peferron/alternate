package main

import (
	"reflect"
	"testing"
	"time"
)

func TestParseArguments(t *testing.T) {
	tests := []struct {
		iOsArgs []string
		oA      arguments
		oErr    string
	}{
		{
			[]string{"alternate"},
			arguments{}, "Not enough arguments",
		},
		{
			[]string{"alternate", "cmd"},
			arguments{}, "Not enough arguments",
		},
		{
			[]string{"alternate", "cmd", "val0"},
			arguments{}, "Not enough arguments",
		},
		{
			[]string{"alternate", "cmd", "val0", "overlap"},
			arguments{}, "Invalid overlap: 'overlap'",
		},
		{
			[]string{"alternate", "cmd", "val0", "5"},
			arguments{}, "Invalid overlap: '5'",
		},
		{
			[]string{"alternate", "cmd", "val0", "-5s"},
			arguments{}, "Invalid overlap: '-5s'",
		},
		{
			[]string{"alternate", "cmd", "val0", "0"},
			arguments{"cmd", []string{"val0"}, 0}, "",
		},
		{
			[]string{"alternate", "cmd", "val0", "5s"},
			arguments{"cmd", []string{"val0"}, 5 * time.Second}, "",
		},
		{
			[]string{"alternate", "cmd", "val0", "123ms"},
			arguments{"cmd", []string{"val0"}, 123 * time.Millisecond}, "",
		},
		{
			[]string{"alternate", "cmd", "val0", "val%1", "val 2", "", "0"},
			arguments{"cmd", []string{"val0", "val%1", "val 2", ""}, 0}, "",
		},
	}

	for i, test := range tests {
		a, err := parseArguments(test.iOsArgs)
		if !sameError(err, test.oErr) {
			t.Errorf("For test #%d with osArgs %v, expected err to be '%s', but was '%s'",
				i, test.iOsArgs, test.oErr, err)
		}
		if !reflect.DeepEqual(test.oA, a) {
			t.Errorf("For test #%d with osArgs %v, expected a to be %+v, but was %+v",
				i, test.iOsArgs, test.oA, a)
		}
	}
}

func sameError(a error, b string) bool {
	if a == nil {
		return b == ""
	}
	return a.Error() == b
}
