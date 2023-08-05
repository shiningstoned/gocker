package main

import (
	"fmt"
	"golang.org/x/sys/unix"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func createContainerId() string {
	randBytes := make([]byte, 6)
	rand.Read(randBytes)
	return fmt.Sprintf("%02x%02x%02x%02x%02x%02x",
		randBytes[0], randBytes[1], randBytes[2], randBytes[3], randBytes[4], randBytes[5])
}

func createContainerDirs(containerId string) {
	contHome := getGockerContainersPath() + "/" + containerId
	dirs := []string{contHome + "/fs", contHome + "/fs/mnt", contHome + "/fs/upperdir", contHome + "/fs/workdir"}
	if err := createDirsIfNotExist(dirs); err != nil {
		log.Fatalln("create container dirs failed")
	}
}

func mountOverlayFileSystem(containerId, imageShaHex string) {
	var srcLayers []string
	pathManifest := getManifestPathForImage(imageShaHex)
	mani := manifest{}
	parseManifest(pathManifest, &mani)
	if len(mani) == 0 || len(mani[0].Layers) == 0 {
		log.Fatal("Could not find any layers.")
	}
	if len(mani) > 1 {
		log.Fatalln("could not handle more than one manifest")
	}

	imageBasePath := getGockerImagesPath() + "/" + imageShaHex
	for _, layer := range mani[0].Layers {
		srcLayers = append(srcLayers, imageBasePath+"/"+layer[:12]+"/fs")
	}
	contFSHome := getContainerFSHome(containerId)
	mntOptions := "lowerdir=" + strings.Join(srcLayers, ":") + ",upperdir=" + contFSHome + "/upperdir,workdir=" + contFSHome + "/workdir"
	if err := unix.Mount("none", contFSHome+"/mnt", "overlay", 0, mntOptions); err != nil {
		log.Fatalf("mount container file system failed: %v", err)
	}
}

func getContainerFSHome(containerId string) string {
	return getGockerContainersPath() + "/" + containerId + "/fs"
}

func prepareAndExecuteContainer(mem int, swap int, pids int, cpus float64,
	containerId string, imageShaHex string, args []string) {
	cmd := &exec.Cmd{
		Path:   "/proc/self/exe",
		Args:   []string{"/proc/self/exe", "setup-netns", containerId},
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	cmd.Run()

	cmd = &exec.Cmd{
		Path:   "/proc/self/exe",
		Args:   []string{"/proc/self/exe", "setup-veth", containerId},
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	cmd.Run()

	var options []string
	if mem > 0 {
		options = append(options, "--mem="+strconv.Itoa(mem))
	}
	if swap > 0 {
		options = append(options, "--swap="+strconv.Itoa(swap))
	}
	if pids > 0 {
		options = append(options, "--pids="+strconv.Itoa(pids))
	}
	if cpus > 0 {
		options = append(options, "--cpus="+strconv.FormatFloat(cpus, 'f', 1, 64))
	}
	options = append(options, "--img="+imageShaHex)
	args = append([]string{containerId}, args...)
	args = append(options, args...)
	args = append([]string{"child-mode"}, args...)
	cmd = exec.Command("/proc/self/exe", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &unix.SysProcAttr{
		Cloneflags: unix.CLONE_NEWPID | unix.CLONE_NEWNS | unix.CLONE_NEWIPC | unix.CLONE_NEWUTS,
	}
	cmd.Run()
}

func unmountContainerFS(containerId string) {
	path := getGockerContainersPath() + "/" + containerId + "/fs/mnt"
	if err := unix.Unmount(path, 0); err != nil {
		log.Fatalln("unmount container file system failed")
	}
}

func unmountNetWorkNamespace(containerId string) {
	path := getGockerNetNsPath() + "/" + containerId
	if err := unix.Unmount(path, 0); err != nil {
		log.Fatalln("unmount container network namespace failed")
	}
}

func execContainerCommand(mem int, swap int, pids int, cpus float64, containerId string, imageShaHex string, args []string) {
	mntPath := getContainerFSHome(containerId) + "/mnt"
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	imgConfig := parseContainerConfig(imageShaHex)
	doOrDieWithMsg(unix.Sethostname([]byte(containerId)), "set container hostname failed")
	doOrDieWithMsg(joinContainerNetworkNamespace(containerId), "join container Netns failed")
	createCGroups(containerId, true)
	configureCGroups(containerId, mem, swap, pids, cpus)
	doOrDieWithMsg(copyNameserverConfig(containerId), "copy resolv.conf failed")
	doOrDieWithMsg(unix.Chroot(mntPath), "change root failed")
	doOrDieWithMsg(os.Chdir("/"), "change dir failed")
	createDirsIfNotExist([]string{"/proc", "/sys"})
	doOrDieWithMsg(unix.Mount("proc", "/proc", "proc", 0, ""), "mount proc failed")
	doOrDieWithMsg(unix.Mount("tmpfs", "/tmp", "tmpfs", 0, ""), "mount tmpfs failed")
	doOrDieWithMsg(unix.Mount("tmpfs", "/dev", "tmpfs", 0, ""), "mount tmpfs on /dev failed")
	createDirsIfNotExist([]string{"/dev/pts"})
	doOrDieWithMsg(unix.Mount("devpts", "/dev/pts", "devpts", 0, ""), "mount devpts failed")
	doOrDieWithMsg(unix.Mount("sysfs", "/sys", "sysfs", 0, ""), "mount sysfs failed")
	setupLocalInterface()
	cmd.Env = imgConfig.Config.Env
	cmd.Run()
	doOrDie(unix.Unmount("/dev/pts", 0))
	doOrDie(unix.Unmount("/dev", 0))
	doOrDie(unix.Unmount("/sys", 0))
	doOrDie(unix.Unmount("/proc", 0))
	doOrDie(unix.Unmount("/tmp", 0))
}

func copyNameserverConfig(containerId string) error {
	resolvFilePaths := []string{"/var/run/systemd/resolve/resolv.conf",
		"/etc/resolv.conf"}
	for _, resolvFilePath := range resolvFilePaths {
		if _, err := os.Stat(resolvFilePath); os.IsNotExist(err) {
			continue
		} else {

			return copyFile(resolvFilePath, getContainerFSHome(containerId)+"/mnt/etc/resolv.conf")
		}
	}
	return nil
}

func initContainer(mem int, swap int, pids int, cpus float64, src string, args []string) {
	containerId := createContainerId()
	log.Printf("container Id %s\n", containerId)
	imageShaHex := downloadImageIfRequired(src)
	log.Printf("image to overlay mount %s\n", imageShaHex)
	createContainerDirs(containerId)
	mountOverlayFileSystem(containerId, imageShaHex)
	if err := setupVirtualEthOnHost(containerId); err != nil {
		log.Fatalln("set up veth0 failed")
	}
	prepareAndExecuteContainer(mem, swap, pids, cpus, containerId, imageShaHex, args)
	log.Println("container done")
	unmountNetWorkNamespace(containerId)
	unmountContainerFS(containerId)
	removeCGroups(containerId)
	os.RemoveAll(getGockerContainersPath() + "/" + containerId)
}
