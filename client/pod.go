package client

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeletePod deletes the pod
func (cli *Client) DeletePod(pod *corev1.Pod) error {
	return cli.podInterface.Delete(pod.Name, &metav1.DeleteOptions{})
}

// GetExternalIPOfPod returns the external IP address of node hosting the pod
func (cli *Client) GetExternalIPOfPod(pod *corev1.Pod) (string, error) {
	node, err := cli.nodeInterface.Get(pod.Spec.NodeName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	for _, addr := range node.Status.Addresses {
		if addr.Type == corev1.NodeExternalIP {
			return addr.Address, nil
		}
	}
	return "", fmt.Errorf("no external address found in node %s", node.Name)
}
