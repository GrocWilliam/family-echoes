//go:build mage

//mage:multiline

// Set the general description you want to have displayed with mage -l here.
package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/magefile/mage/mg" // mg contains helpful utility functions, like Deps
)

// Default target to run when none is specified
// If not set, running mage will list available targets
// var Default = Build

// A build step that requires additional params, or platform specific steps for example
func Build() error {
	mg.Deps(InstallDeps)
	fmt.Println("Building...")
	cmd := exec.Command("go", "build", "-o", "Echoes", ".")
	return cmd.Run()
}

// A custom install step if you need your bin someplace other than go/bin
func Install() error {
	mg.Deps(Build)
	fmt.Println("Installing...")
	return os.Rename("./Echoes", "/usr/bin/Echoes")
}

func Dev() error {
	fmt.Println("Running...")
	cmd := exec.Command("air", "main.go")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Manage your deps, or running package managers.
func InstallDeps() error {
	fmt.Println("Installing Deps...")
	cmd := exec.Command("go", "get")
	return cmd.Run()
}

// Clean up after yourself
func Clean() {
	fmt.Println("Cleaning...")
	os.RemoveAll("Echoes")
}
