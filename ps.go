package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type runningContainerInfo struct {
	containerId string
	image       string
	command     string
	pid         int
}

func getRunningContainers() ([]runningContainerInfo, error) {
	var containers []runningContainerInfo
	path := "/sys/fs/cgroup/cpu/gocker"

	entries, err := os.ReadDir(path)
	if os.IsNotExist(err) {
		return containers, err
	} else {
		for _, entry := range entries {
			if entry.IsDir() {
				container, _ := getRunningContainerInfoForId(entry.Name())
				if container.pid > 0 {
					containers = append(containers, container)
				}
			}
		}
		return containers, nil
	}
}

func getRunningContainerInfoForId(containerId string) (runningContainerInfo, error) {
	container := runningContainerInfo{}
	var procs []string
	path := "/sys/fs/cgroup/cpu/gocker"

	file, err := os.Open(path + "/" + containerId + "/cgroup.procs")
	if err != nil {
		return container, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		procs = append(procs, scanner.Text())
	}
	if len(procs) > 0 {
		pid, err := strconv.Atoi(procs[len(procs)-1])
		if err != nil {
			return container, err
		}
		cmd, err := os.Readlink("/proc/" + strconv.Itoa(pid) + "/exe")
		if err != nil {
			log.Println("read command link failed")
		}
		containerMntPath := getGockerContainersPath() + "/" + containerId + "/fs/mnt"
		realContainerMntPath, err := filepath.EvalSymlinks(containerMntPath)
		if err != nil {
			log.Println("resolve path failed")
		}

		image, _ := getDistribution(containerId)
		container = runningContainerInfo{
			containerId: containerId,
			image:       image,
			command:     cmd[len(realContainerMntPath):],
			pid:         pid,
		}
	}
	return container, nil
}

func getDistribution(containerId string) (string, error) {
	var lines []string
	file, err := os.Open("/proc/mounts")
	if err != nil {
		log.Println("read /proc/mounts failed")
		return "", err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	for _, line := range lines {
		if strings.Contains(line, containerId) {
			parts := strings.Split(line, " ")
			for _, part := range parts {
				if strings.Contains(part, "lowerdir=") {
					options := strings.Split(part, ",")
					for _, option := range options {
						if strings.Contains(option, "lowerdir=") {
							imagePath := getGockerImagesPath()
							leaderString := "lowerdir=" + imagePath + "/"
							trailerString := option[len(leaderString):]
							imageId := trailerString[:12]
							imgName, tagName := imageExistByHash(imageId)
							return fmt.Sprintf("%s:%s", imgName, tagName), nil
						}
					}
				}
			}
		}
	}
	return "", nil
}

func printRunningContainers() {
	containers, err := getRunningContainers()
	if err != nil {
		log.Fatalln("get running containers failed")
	}

	fmt.Println("CONTAINER ID\tIMAGE\t\tCOMMAND")
	for _, container := range containers {
		fmt.Printf("%s\t%s\t\t%s\n", container.containerId, container.image, container.command)
	}
}
