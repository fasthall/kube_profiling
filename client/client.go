package client

import (
	"k8s.io/client-go/kubernetes"
	typedbatchv1 "k8s.io/client-go/kubernetes/typed/batch/v1"
	clientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"
)

// Client is a wrapper which provides several functions leveraging client-go API
type Client struct {
	nodeInterface clientv1.NodeInterface
	jobInterface  typedbatchv1.JobInterface
	podInterface  clientv1.PodInterface
	sshKey        string
}

// NewClient returns a new NodeClient instance
func NewClient(kubeconfigPath, namespace, sshKey string) (*Client, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &Client{
		nodeInterface: clientset.CoreV1().Nodes(),
		jobInterface:  clientset.BatchV1().Jobs(namespace),
		podInterface:  clientset.CoreV1().Pods(namespace),
		sshKey:        sshKey,
	}, nil
}
