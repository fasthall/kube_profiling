package util

import (
	"bytes"
	"log"
	"os/exec"
	"os/user"
)

// HomeDir returns the current user's home directory
func HomeDir() string {
	usr, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}
	return usr.HomeDir
}

// RunSSHCommand ssh into the host and run a command using sh -c
func RunSSHCommand(addr, key string, command []string) (string, string, error) {
	args := append([]string{"-i", key, addr}, command...)
	cmd := exec.Command("ssh", args...)
	var bufOut bytes.Buffer
	var bufErr bytes.Buffer
	cmd.Stdout = &bufOut
	cmd.Stderr = &bufErr
	err := cmd.Run()
	if err != nil {
		return "", "", err
	}

	return bufOut.String(), bufErr.String(), nil
}

// RunSCPCommand runs scp to copy a file to the host
func RunSCPCommand(key, src, dst string) (string, string, error) {
	cmd := exec.Command("scp", "-i", key, src, dst)
	var bufOut bytes.Buffer
	var bufErr bytes.Buffer
	cmd.Stdout = &bufOut
	cmd.Stderr = &bufErr
	err := cmd.Run()
	if err != nil {
		return "", "", err
	}

	return bufOut.String(), bufErr.String(), nil
}
