package main

import (
	flag "github.com/spf13/pflag"
	"log"
	"math/rand"
	"os"
	"time"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	options := []string{"run", "child-mode", "setup-netns", "setup-veth", "ps", "exec", "images", "rmi"}

	if len(os.Args) < 2 || !stringInSlice(os.Args[1], options) {
		usage()
		os.Exit(1)
	}

	if os.Getuid() != 0 {
		log.Fatalf("you need root privilege to run this program")
	}

	if err := initGockerDirs(); err != nil {
		log.Fatalf("init gocker directories failed: %v\n", err)
	}

	switch os.Args[1] {
	case "run":
		fs := flag.FlagSet{}
		fs.ParseErrorsWhitelist.UnknownFlags = true

		mem := fs.Int("mem", -1, "Max RAM to allow in MB")
		swap := fs.Int("swap", -1, "Max swap to allow in MB")
		pids := fs.Int("pids", -1, "Max number of processes to allow")
		cpus := fs.Float64("cpus", -1, "Number of cpu core to restrict to ")
		if err := fs.Parse(os.Args[2:]); err != nil {
			log.Fatalf("parse arguments failed: %v\n", err)
		}
		if len(fs.Args()) < 2 {
			log.Fatalln("image name and commands are needed")
		}

		if isUp, _ := isGockerBridgeUp(); !isUp {
			if err := setupGockerBridge(); err != nil {
				log.Fatalf("set up gocker bridge failed: %v\n", err)
			}
		}

		initContainer(*mem, *swap, *pids, *cpus, fs.Args()[0], fs.Args()[1:])
	case "child-mode":
		fs := flag.FlagSet{}
		fs.ParseErrorsWhitelist.UnknownFlags = true

		mem := fs.Int("mem", -1, "Max RAM to allow in MB")
		swap := fs.Int("swap", -1, "Max swap to allow in MB")
		pids := fs.Int("pids", -1, "Max number of processes to allow")
		cpus := fs.Float64("cpus", -1, "Number of cpu core to restrict to ")
		image := fs.String("img", "", "container image")
		if err := fs.Parse(os.Args[2:]); err != nil {
			log.Fatalln("parse arguments failed")
		}
		if len(fs.Args()) < 2 {
			log.Fatalln("image and command are needed")
		}
		execContainerCommand(*mem, *swap, *pids, *cpus, fs.Args()[0], *image, fs.Args()[1:])
	case "setup-netns":
		setupNewNetworkNamespace(os.Args[2])
	case "setup-veth":
		setupContainerNetworkInterfaceStep1(os.Args[2])
		setupContainerNetworkInterfaceStep2(os.Args[2])
	case "ps":
		printRunningContainers()
	case "rmi":
		if len(os.Args) < 3 {
			usage()
			os.Exit(1)
		}
		deleteImageByHash(os.Args[2])
	case "exec":
		execInContainer(os.Args[2])
	case "images":
		printAvailableImages()
	default:
		usage()
	}
}
