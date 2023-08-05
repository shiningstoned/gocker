package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
)

const (
	gockerHomePath       = "/var/lib/gocker"
	gockerImagesPath     = gockerHomePath + "/images"
	gockerTempPath       = gockerHomePath + "/tmp"
	gockerContainersPath = "/var/run/gocker/containers"
	gockerNetNsPath      = "/var/run/gocker/net-ns"
)

func usage() {
	fmt.Println("Welcome to gocker!")
	fmt.Println("Supported commands:")
	fmt.Println("gocker run [--mem] [--swap] [--pids] [--cpus] <image> <commands>")
	fmt.Println("gocker exec <container-id> <commands>")
	fmt.Println("gocker images")
	fmt.Println("gocker ps")
	fmt.Println("gocker rmi <image-id>")
}

func initGockerDirs() error {
	dirs := []string{gockerHomePath, gockerTempPath, gockerImagesPath, gockerContainersPath}
	return createDirsIfNotExist(dirs)
}

func createDirsIfNotExist(dirs []string) error {
	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return err
			}
		}
	}
	return nil
}

func stringInSlice(target string, list []string) bool {
	for _, s := range list {
		if target == s {
			return true
		}
	}
	return false
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}

func doOrDie(err error) {
	if err != nil {
		log.Fatalf("error: %v", err)
	}
}

func doOrDieWithMsg(err error, msg string) {
	if err != nil {
		log.Fatalf("%v: %v", msg, err)
	}
}

func parseManifest(manifestPath string, mani *manifest) error {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, mani); err != nil {
		return err
	}
	return nil
}

func getGockerImagesPath() string {
	return gockerImagesPath
}

func getGockerTempPath() string {
	return gockerTempPath
}

func getGockerContainersPath() string {
	return gockerContainersPath
}

func getGockerNetNsPath() string {
	return gockerNetNsPath
}
