package main

import (
	"golang.org/x/sys/unix"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func getPidForRunningContainer(containerId string) int {
	containers, err := getRunningContainers()
	if err != nil {
		log.Fatalln("get running container failed")
	}
	for _, container := range containers {
		if container.containerId == containerId {
			return container.pid
		}
	}
	return 0
}

func execInContainer(containerId string) {
	pid := getPidForRunningContainer(containerId)
	if pid == 0 {
		log.Fatalln("no such container")
	}
	baseNsPath := "/proc/" + strconv.Itoa(pid) + "/exe"
	ipcFd, ipcErr := os.Open(baseNsPath + "/ipc")
	mntFd, mntErr := os.Open(baseNsPath + "/mnt")
	netFd, netErr := os.Open(baseNsPath + "/net")
	pidFd, pidErr := os.Open(baseNsPath + "/pid")
	utsFd, utsErr := os.Open(baseNsPath + "/uts")

	if ipcErr != nil || mntErr != nil || netErr != nil || pidErr != nil || utsErr != nil {
		log.Fatalln("open namespace file failed")
	}

	unix.Setns(int(ipcFd.Fd()), unix.CLONE_NEWIPC)
	unix.Setns(int(mntFd.Fd()), unix.CLONE_NEWNS)
	unix.Setns(int(netFd.Fd()), unix.CLONE_NEWNET)
	unix.Setns(int(pidFd.Fd()), unix.CLONE_NEWPID)
	unix.Setns(int(utsFd.Fd()), unix.CLONE_NEWUTS)

	container, err := getRunningContainerInfoForId(containerId)
	if err != nil {
		log.Fatalln("get running container info failed")
	}
	imgNameAndHash := strings.Split(container.image, ":")
	exist, imageShaHex := imageExistByTag(imgNameAndHash[0], imgNameAndHash[1])
	if !exist {
		log.Fatalln("get image hash failed")
	}
	imgConfig := parseContainerConfig(imageShaHex)
	containerMntPath := getGockerContainersPath() + "/" + containerId + "/fs/mnt"
	createCGroups(containerId, false)
	unix.Chroot(containerMntPath)
	os.Chdir("/")
	cmd := exec.Command(os.Args[3], os.Args[4:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = imgConfig.Config.Env
	cmd.Run()
}
