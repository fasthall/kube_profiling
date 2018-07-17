package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/fasthall/kubeprof/client"
	"github.com/fasthall/kubeprof/util"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

var tool string
var kubeConfig string
var sshKey string
var stageDir string
var jobFile string
var namespace string
var skipChecking bool

func init() {
	flag.StringVar(&tool, "tool", "", "tool of choice to profile the workload")
	flag.StringVar(&jobFile, "job_file", "", "job description JSON file")
	flag.StringVar(&kubeConfig, "kube_config", filepath.Join(util.HomeDir(), ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	flag.StringVar(&sshKey, "ssh_key", filepath.Join(util.HomeDir(), ".ssh", "google_compute_engine"), "(optional) absolute path to the ssh private key")
	flag.StringVar(&stageDir, "stage_dir", "/tmp/", "(optional) node directory to temporarily store profiling tool binary")
	flag.StringVar(&namespace, "namespace", "default", "(optional) Kubernetes namespace to run the job")
	flag.BoolVar(&skipChecking, "skip_checking", false, "(optional) skip checking if binary file is ready")
	flag.Parse()
}

func main() {
	if tool == "" {
		log.Panicf("Profiling tool not specified. Choose from [perf]")
		return
	}
	if jobFile == "" {
		log.Panicln("Please specify the path of job description file.")
		return
	}
	cli, err := client.NewClient(kubeConfig, namespace, sshKey)
	if err != nil {
		log.Panicf("Couldn't create a node client with config file %s.\n", kubeConfig)
		return
	}
	if !skipChecking {
		addresses, err := cli.ListExternalIPs()
		if err != nil {
			log.Println("Couldn't list external IPs of nodes.")
			panic(err)
		}
		for _, addr := range addresses {
			log.Printf("Checking if %s can be found in %s...\n", tool, addr)
			exist, err := cli.CheckBinary(addr, stageDir+tool)
			if err != nil {
				log.Printf("Couldn't connect to the host %s.\n", addr)
			} else {
				if !exist {
					log.Printf("%s couldn't be found. Uploading the binary...\n", tool)
					cwd, err := os.Getwd()
					if err != nil {
						log.Panicln("Couldn't get the current workdir.")
					}
					source := cwd + "/bin/" + tool
					err = cli.UploadBinary(addr, source, stageDir)
					if err != nil {
						log.Printf("Couldn't upload %s to host %s.\n", tool, addr)
						log.Panicln("Make the user has the permission to copy file into the stage directory.")
					}
					log.Println(tool, "uploaded.")
				}
			}
		}
		log.Println(tool, "are found in all nodes. Creating Kubernetes job...")
	} else {
		log.Println("Skip checking if binary file is ready on all nodes.")
	}

	log.Printf("Loading job description file %s.\n", jobFile)
	jobObj := client.ParseFromJSON(jobFile)
	jobObj = client.AddSecurityContext(jobObj)
	jobObj = client.AddFileMount(jobObj, tool, stageDir+tool)
	cmds, err := util.GetImageCommand("fasthall/iperf")
	if err != nil {
		log.Panicln("Failed to get the original command of Docker image.")
	}
	jobObj = client.OverrideCommand(jobObj, tool, stageDir+tool, cmds)
	jobObj, err = cli.CreateJob(jobObj)
	if err != nil {
		log.Panicln("Failed to create job.", err)
	}
}
