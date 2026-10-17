//go:build darwin || linux

package process

import (
	"os"
	"reflect"
	"testing"
)

func TestReadOwnProcessArgv(t *testing.T) {
	args, err := processArgv(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(args, os.Args) {
		t.Fatalf("arguments did not preserve OS boundaries")
	}
}
