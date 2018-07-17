package client

import (
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateJob creates a job
func (cli *Client) CreateJob(job *batchv1.Job) (*batchv1.Job, error) {
	return cli.jobInterface.Create(job)
}

// WaitForJobComplete blocks until the job completes
func (cli *Client) WaitForJobComplete(job *batchv1.Job) error {
	jobName := job.Name
	for {
		job, err := cli.jobInterface.Get(jobName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if job.Status.CompletionTime != nil {
			return nil
		}
		time.Sleep(1 * time.Second)
	}
}

// DeleteJobSync deletes a job
func (cli *Client) DeleteJobSync(job *batchv1.Job) error {
	err := cli.jobInterface.Delete(job.Name, &metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}

// GetPodsOfJob returns a list of jobs created by the job
func (cli *Client) GetPodsOfJob(job *batchv1.Job) ([]corev1.Pod, error) {
	job, err := cli.jobInterface.Get(job.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	labels := job.Spec.Template.Labels
	labelSelector := []string{}
	for k, v := range labels {
		labelSelector = append(labelSelector, k+"="+v)
	}
	podList, err := cli.podInterface.List(metav1.ListOptions{
		LabelSelector: strings.Join(labelSelector, ","),
	})
	if err != nil {
		return nil, err
	}
	return podList.Items, nil
}
