//go:build !darwin && !linux

package service

import "fmt"

func Installed() (bool, error) {
	return false, nil
}

func Running() (bool, error) {
	return false, nil
}

func Install(exe string) error {
	return fmt.Errorf("background service install is only supported on macOS and Linux")
}

func Remove() error {
	return fmt.Errorf("background service remove is only supported on macOS and Linux")
}

func Describe() (installed, running bool, hint string, err error) {
	return false, false, "Service install/remove is only available on macOS and Linux.", nil
}
