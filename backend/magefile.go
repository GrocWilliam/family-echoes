//go:build mage

package main

import (
	"errors"
	"os"

	"github.com/magefile/mage/sh"
)

// Build compiles the API into ./Echoes.
func Build() error {
	return sh.RunV("go", "build", "-o", "Echoes", ".")
}

// Dev runs the API with hot reload (requires the air binary).
func Dev() error {
	return sh.RunV("air", "main.go")
}

// Test runs the test suite.
func Test() error {
	return sh.RunV("go", "test", "./...")
}

// Check runs gofmt, go vet and the tests — the same checks as CI.
func Check() error {
	unformatted, err := sh.Output("gofmt", "-l", ".")
	if err != nil {
		return err
	}
	if unformatted != "" {
		return errors.New("gofmt needed on:\n" + unformatted)
	}
	if err := sh.RunV("go", "vet", "./..."); err != nil {
		return err
	}
	return Test()
}

// Deps downloads module dependencies.
func Deps() error {
	return sh.RunV("go", "mod", "download")
}

// Clean removes the build output.
func Clean() error {
	return os.RemoveAll("Echoes")
}
