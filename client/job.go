package client

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
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

// ParseFromJSON reads from a JSON job description file and returns a Job object
func ParseFromJSON(jsonfile string) *batchv1.Job {
	file, err := os.Open(jsonfile)
	if err != nil {
		log.Panicf("Failed to open job description file %s.\n", jsonfile)
	}
	dec := json.NewDecoder(file)

	// Parse it into the internal k8s structs
	var job batchv1.Job
	dec.Decode(&job)
	return &job
}

// AddSecurityContext takes a job template and add SYS_ADMIN capability and make it run in privileged mode
func AddSecurityContext(jobObj *batchv1.Job) *batchv1.Job {
	privileged := true
	containers := &jobObj.Spec.Template.Spec.Containers
	for i := range *containers {
		if (*containers)[i].SecurityContext == nil {
			(*containers)[i].SecurityContext = &corev1.SecurityContext{}
		}
		if (*containers)[i].SecurityContext.Capabilities == nil {
			(*containers)[i].SecurityContext.Capabilities = &corev1.Capabilities{}
		}
		found := false
		for _, cap := range (*containers)[i].SecurityContext.Capabilities.Add {
			if strings.ToLower(string(cap)) == "sys_admin" {
				found = true
				break
			}
		}
		if !found {
			(*containers)[i].SecurityContext.Capabilities.Add = append((*containers)[i].SecurityContext.Capabilities.Add, "sys_admin")
		}
		(*containers)[i].SecurityContext.Privileged = &privileged
	}
	return jobObj
}

// AddStageDirMount adds the staging directory into container volume mount list
func AddStageDirMount(jobObj *batchv1.Job, stageDir string) *batchv1.Job {
	hostPathType := corev1.HostPathDirectory
	jobObj.Spec.Template.Spec.Volumes = append(jobObj.Spec.Template.Spec.Volumes, corev1.Volume{
		Name: "stage-dir",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: stageDir,
				Type: &hostPathType,
			},
		},
	})
	containers := &jobObj.Spec.Template.Spec.Containers
	for i := range *containers {
		if (*containers)[i].VolumeMounts == nil {
			(*containers)[i].VolumeMounts = []corev1.VolumeMount{}
		}
		(*containers)[i].VolumeMounts = append((*containers)[i].VolumeMounts, corev1.VolumeMount{
			Name:      "stage-dir",
			MountPath: stageDir,
		})
	}
	return jobObj
}

// OverrideCommand overrides the command of containers in job so that it will run profiling tool first and then original command
func OverrideCommand(jobObj *batchv1.Job, tool, stageDir string, originalCmds []string) *batchv1.Job {
	containers := &jobObj.Spec.Template.Spec.Containers
	for i := range *containers {
		(*containers)[i].Command = []string{
			"sh",
			"-c",
			fmt.Sprintf("cp %s %s && perf record -o %s %s && perf report -i %s --stdio > %s",
				filepath.Join(stageDir, tool),
				filepath.Join("/bin/", tool),
				filepath.Join(stageDir, "perf.data"),
				strings.Join(originalCmds, " "),
				filepath.Join(stageDir, "perf.data"),
				filepath.Join(stageDir, "perf.report")),
		}
	}
	return jobObj
}
