package main

import (
	"log"
	"os"
	"runtime"
	"strconv"
)

func removeCGroups(containerId string) {
	cgroups := []string{"/sys/fs/cgroup/memory/gocker/" + containerId,
		"/sys/fs/cgroup/pids/gocker/" + containerId,
		"/sys/fs/cgroup/cpu/gocker/" + containerId}

	for _, dir := range cgroups {
		if err := os.Remove(dir); err != nil {
			log.Fatalf("remove container cgroup dir failed %v", err)
		}
	}
}

func setMemoryLimit(containerId string, mem int, swap int) {
	memFilePath := "/sys/fs/cgroup/memory/gocker/" + containerId + "/memory.limit_in_bytes"
	swapFilePath := "/sys/fs/cgroup/memory/gocker/" + containerId + "/memory.memsw.limit_in_bytes"
	doOrDieWithMsg(os.WriteFile(memFilePath, []byte(strconv.Itoa(mem*1024*1024)), 0644), "write memory file failed")

	if swap > 0 {
		doOrDieWithMsg(os.WriteFile(swapFilePath, []byte(strconv.Itoa(mem*1024*1024+swap*1024*1024)), 0644), "write swap file failed")
	}
}

func setCpuLimit(containerId string, cpus float64) {
	cfsPeriodPath := "/sys/fs/cgroup/cpu/gocker/" + containerId + "/cpu.cfs_period_us"
	cfsQuotaPath := "/sys/fs/cgroup/cpu/gocker/" + containerId + "/cpu.cfs_quota_us"

	if cpus > float64(runtime.NumCPU()) {
		log.Println("ignore to set cpu quota greater than available cpus")
	}

	doOrDieWithMsg(os.WriteFile(cfsPeriodPath,
		[]byte(strconv.Itoa(1000000)), 0644),
		"Unable to write CFS period")

	doOrDieWithMsg(os.WriteFile(cfsQuotaPath,
		[]byte(strconv.Itoa(int(1000000*cpus))), 0644),
		"Unable to write CFS period")
}

func setPidsLimit(containerID string, pids int) {
	maxProcsPath := "/sys/fs/cgroup/pids/gocker/" + containerID +
		"/pids.max"

	doOrDieWithMsg(os.WriteFile(maxProcsPath,
		[]byte(strconv.Itoa(pids)), 0644),
		"Unable to write pids limit")

}

func configureCGroups(containerId string, mem int, swap int, pids int, cpus float64) {
	if mem > 0 {
		setMemoryLimit(containerId, mem, swap)
	}
	if pids > 0 {
		setPidsLimit(containerId, pids)
	}
	if cpus > 0 {
		setCpuLimit(containerId, cpus)
	}
}

func createCGroups(containerId string, createCGroupDirs bool) {
	cgroups := []string{"/sys/fs/cgroup/memory/gocker/" + containerId,
		"/sys/fs/cgroup/pids/gocker/" + containerId,
		"/sys/fs/cgroup/cpu/gocker/" + containerId}
	if createCGroupDirs {
		if err := createDirsIfNotExist(cgroups); err != nil {
			log.Fatalln("create cgroup dirs failed")
		}
	}

	for _, cgroupDir := range cgroups {
		if err := os.WriteFile(cgroupDir+"/notify_on_release", []byte("1"), 0700); err != nil {
			log.Fatalln("write cgroup file failed")
		}
		if err := os.WriteFile(cgroupDir+"/cgroup.procs", []byte(strconv.Itoa(os.Getpid())), 0700); err != nil {
			log.Fatalln("write cgroup file failed")
		}
	}
}
