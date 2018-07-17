package util

import (
	"os/exec"
	"strings"
)

// GetImageCommand checks the default command of Docker image
// It pulls the image first then inspect it
func GetImageCommand(image string) ([]string, error) {
	cmd := exec.Command("docker", "pull", image)
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	cmd = exec.Command("docker", "inspect", "--format={{.Config.Cmd}}", image)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return strings.Split(string(out)[1:len(out)-2], " "), nil
}
