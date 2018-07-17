package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/fasthall/kubeprof/client"
	"github.com/fasthall/kubeprof/util"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

var tool string
var kubeConfig string
var sshKey string
var stageDir string
var outputDir string
var jobFile string
var namespace string
var skipChecking bool
var deleteJob bool

func init() {
	flag.StringVar(&tool, "tool", "", "tool of choice to profile the workload")
	flag.StringVar(&jobFile, "job_file", "", "job description JSON file")
	flag.StringVar(&kubeConfig, "kube_config", filepath.Join(util.HomeDir(), ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	flag.StringVar(&sshKey, "ssh_key", filepath.Join(util.HomeDir(), ".ssh", "google_compute_engine"), "(optional) absolute path to the ssh private key")
	flag.StringVar(&stageDir, "stage_dir", "/tmp/", "(optional) node directory to temporarily store profiling tool binary")
	flag.StringVar(&outputDir, "output_dir", "", "(optional) host directory to store result files")
	flag.StringVar(&namespace, "namespace", "default", "(optional) Kubernetes namespace to run the job")
	flag.BoolVar(&skipChecking, "skip_checking", false, "skip checking if binary file is ready")
	flag.BoolVar(&deleteJob, "rm", false, "delete the job and the associated pods after finished")
	flag.Parse()
}

func main() {
	if tool == "" {
		log.Fatalf("Profiling tool not specified. Choose from [perf]")
	}

	if jobFile == "" {
		log.Fatalln("Please specify the path of job description file.")
	}

	if outputDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			log.Fatalln("Couldn't get working directory. Please specify output directory with option --output_dir.")
		}
		outputDir = wd
	} else {
		// create the output directory if not exist
		if _, err := os.Stat(outputDir); os.IsNotExist(err) {
			os.Mkdir(outputDir, 0755)
		}
	}
	// make a subdirectory by the current time
	now := time.Now()
	outputDir = filepath.Join(outputDir, fmt.Sprintf("%d%02d%02dT%02d%02d%02d", now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), now.Second()))
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		os.Mkdir(outputDir, 0755)
	}

	cli, err := client.NewClient(kubeConfig, namespace, sshKey)
	if err != nil {
		log.Fatalf("Couldn't create a node client with config file %s: %v\n", kubeConfig, err)
	}

	if !skipChecking {
		// check if required profiling tool binary is presented on all nodes in cluster
		nodes, err := cli.ListNodes()
		if err != nil {
			log.Fatalln("Couldn't list nodes of cluster:", err)
		}
		for _, node := range nodes {
			log.Printf("Checking if %s can be found in %s...\n", tool, node.Name)
			exist, err := cli.CheckBinary(node, filepath.Join(stageDir, tool))
			if err != nil {
				log.Fatalf("Couldn't connect to the node %s: %v\n", node.Name, err)
			} else {
				if !exist {
					// if the binary is not found, upload it to the stage directory
					log.Printf("%s couldn't be found. Uploading the binary...\n", tool)
					cwd, err := os.Getwd()
					if err != nil {
						log.Fatalln("Couldn't get the current workdir:", err)
					}
					err = cli.UploadBinary(node, filepath.Join(cwd, "bin/", tool), stageDir)
					if err != nil {
						log.Fatalf("Couldn't upload %s to node %s: %v.\nMake the user has the permission to copy file into the stage directory.\n", tool, node.Name, err)
					}
					log.Println(tool, "uploaded.")
				}
			}
			err = cli.EnableKernelSymbols(node)
			if err != nil {
				log.Printf("Failed to enables kernel symbol on node %s.\n", node.Name)
			}
		}
		log.Println(tool, "are found in all nodes. Creating Kubernetes job...")
	} else {
		log.Println("Skip checking if binary file is ready on all nodes.")
	}

	log.Printf("Loading job description file %s.\n", jobFile)
	jobObj := util.ParseFromJSON(jobFile)
	// make the job runs in privileged mode and given SYS_ADMIN capability for perf to work properly
	jobObj = util.AddSecurityContext(jobObj)
	// mount the profiling tool binary from node host to pod
	jobObj = util.AddStageDirMount(jobObj, stageDir)

	if len(jobObj.Spec.Template.Spec.Containers) == 0 {
		log.Println("No container specified in job description.")
		return
	}
	cmds, err := util.GetJobCommand(jobObj)
	if err != nil {
		// TODO run it and inspect the image on the node to save space and eliminate the need to install Docker on the host machine
		// 	    the following command pulls the image to the host (not the node) and check its original command by docker inspect,
		// 	    hence it needs the host machine has Docker installed and running
		// 	    also it takes space to pull the image to the host
		cmds, err = util.GetImageCommand(jobObj.Spec.Template.Spec.Containers[0].Image)
		if err != nil {
			log.Fatalf("Failed to get the original command of Docker image %s.\n", jobObj.Spec.Template.Spec.Containers[0].Image)
		}
	}
	// override the pod command so it copies and runs profiling tool first followed by the original command
	jobObj = util.OverrideCommand(jobObj, tool, stageDir, cmds)
	log.Printf("Creating job %s...\n", jobObj.Name)

	jobObj, err = cli.CreateJob(jobObj)
	if err != nil {
		log.Fatalln("Failed to create job:", err)
	}
	log.Printf("Job %s created.\n", jobObj.Name)

	log.Println("Waiting for job to complete...")
	err = cli.WaitForJobComplete(jobObj)
	if err != nil {
		log.Fatalln("Couldn't get job's status:", err)
	}

	pods, err := cli.GetPodsOfJob(jobObj)
	if err != nil {
		log.Fatalf("Failed to get the pods of job %s: %v\n", jobObj.Name, err)
	}
	// job is completed, copy result files back to host
	for _, pod := range pods {
		ip, err := cli.GetExternalIPOfPod(&pod)
		if err != nil {
			log.Printf("Couldn't find the external IP address of pod %s: %v\n", pod.Name, err)
		} else {
			// TODO a bit hacky here
			//      perf.data generated by perf in kubernetes has mode 600 rather than usual 644
			//      ssh into the node and run chmod on result files to make sure scp works properly
			util.RunSSHCommand(ip, sshKey, []string{"sudo", "chmod", "644", filepath.Join(stageDir, "perf.data")})
			util.RunSSHCommand(ip, sshKey, []string{"sudo", "chmod", "644", filepath.Join(stageDir, "perf.report")})

			// run scp commands to copy files to the host
			stdout, stderr, err := util.RunSCPCommand(sshKey, ip+":"+filepath.Join(stageDir, "perf.data"), filepath.Join(outputDir, pod.Name+".data"))
			if err != nil {
				log.Printf("Failed to copy perf.data from pod %s: %v\nPlease check if the permission of staging directory %s on node %s is sufficient.\n", pod.Name, err, stageDir, pod.Name)
				log.Println(stdout, stderr)
			}
			stdout, stderr, err = util.RunSCPCommand(sshKey, ip+":"+filepath.Join(stageDir, "perf.report"), filepath.Join(outputDir, pod.Name+".report"))
			if err != nil {
				log.Printf("Failed to copy perf.report from pod %s: %v\nPlease check if the permission of staging directory %s on node %s is sufficient.\n", pod.Name, err, stageDir, pod.Name)
				log.Println(stdout, stderr)
			}
		}
	}

	// if rm flag is set, cleanup job and associated pods
	if deleteJob {
		log.Println("Deleting the job...")
		err = cli.DeleteJobSync(jobObj)
		if err != nil {
			log.Printf("Failed to delete job %s: %v\n", jobObj.Name, err)
		}
		log.Println("Job deleted. Deleting pods...")
		for _, pod := range pods {
			err = cli.DeletePod(&pod)
			if err != nil {
				log.Printf("Failed to delete pod %s: %v\n", pod.Name, err)
			}
		}
		log.Println("Pods deleted.")
	}
}
