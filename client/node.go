package client

import (
	"os/exec"
	"syscall"

	"github.com/fasthall/kubeprof/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ListExternalIPs returns a list of external IPs of all nodes
func (cli *Client) ListExternalIPs() ([]string, error) {
	var addresses []string
	nodeList, err := cli.nodeInterface.List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, node := range nodeList.Items {
		for _, addr := range node.Status.Addresses {
			if addr.Type == corev1.NodeExternalIP {
				addresses = append(addresses, addr.Address)
			}
		}
	}
	return addresses, nil
}

// CheckBinary checks if the binary can be found in the host
func (cli *Client) CheckBinary(host, path string) (bool, error) {
	_, _, err := util.RunSSHCommand(host, cli.sshKey, []string{"which", path})
	if err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				code := status.ExitStatus()
				if code == 1 {
					// exit code 1 means couldn't find the exectuable
					return false, nil
				}
				// other errors mean it failed to connect to the host
				return false, err
			}
		}
	}
	// no error means the binary is located
	return true, nil
}

// UploadBinary uploads the binary file to the host
func (cli *Client) UploadBinary(host, src, dst string) error {
	_, _, err := util.RunSCPCommand(cli.sshKey, src, host+":"+dst)
	if err != nil {
		return err
	}
	return nil
}
