package main

import (
	"encoding/json"
	"fmt"
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"log"
	"os"
	"strings"
)

type manifest []struct {
	Config   string
	RepoTags []string
	Layers   []string
}

type imageConfigDetails struct {
	Env []string `json:"Env"`
	Cmd []string `json:"Cmd"`
}

type imageConfig struct {
	Config imageConfigDetails `json:"config"`
}

type imageEntries map[string]string
type imagesDB map[string]imageEntries

func downloadImageIfRequired(src string) string {
	imgName, tagName := getImageNameAndTag(src)
	if exist, imageShaHex := imageExistByTag(imgName, tagName); !exist {
		log.Printf("downloading metadata for %s:%s, please wait\n", imgName, tagName)
		img, err := crane.Pull(strings.Join([]string{imgName, tagName}, ":"))
		if err != nil {
			log.Fatalf("download image metadata failed: %v", err)
		}

		manifest, _ := img.Manifest()
		imageShaHex = manifest.Config.Digest.Hex[:12]

		log.Println("checking if image exists under another name")
		altImgName, altImgtag := imageExistByHash(imageShaHex)
		if len(altImgtag) > 0 && len(altImgtag) > 0 {
			log.Printf("the image you want is exist as %s:%s\n", altImgName, altImgtag)
			storeImageMetadata(imgName, tagName, imageShaHex)
			return imageShaHex
		} else {
			log.Println("image do not exist, downloading image...")
			downloadImage(img, imageShaHex, src)
			untarFile(imageShaHex)
			processLayerTarballs(imageShaHex, manifest.Config.Digest.Hex)
			storeImageMetadata(imgName, tagName, imageShaHex)
			deleteTempImagePath(imageShaHex)
			return imageShaHex
		}
	} else {
		log.Println("image exist, do not download")
		return imageShaHex
	}
}

func processLayerTarballs(imageShaHex string, fullImageHex string) {
	path := getGockerTempPath() + "/" + imageShaHex
	pathManifest := path + "/manifest.json"
	pathConfig := path + "/" + fullImageHex + ".json"

	mani := manifest{}
	parseManifest(pathManifest, &mani)
	if len(mani) == 0 || len(mani[0].Layers) == 0 {
		log.Fatal("Could not find any layers")
	}
	if len(mani) > 1 {
		log.Fatalln("could not handle more than one manifest")
	}

	imageDir := getGockerImagesPath() + "/" + imageShaHex
	err := os.Mkdir(imageDir, 0755)
	if err != nil {
		log.Fatalln(err)
	}

	for _, layer := range mani[0].Layers {
		imageLayerDir := imageDir + "/" + layer[:12] + "/fs"
		err = os.MkdirAll(imageLayerDir, 0755)
		if err != nil {
			log.Fatalln(err)
		}
		srcLayer := path + "/" + layer
		if err := untar(srcLayer, imageLayerDir); err != nil {
			log.Fatalln("unable to untar layer file")
		}
	}
	copyFile(pathManifest, getManifestPathForImage(imageShaHex))
	copyFile(pathConfig, getConfigPathForImage(imageShaHex))
}

func parseContainerConfig(imageShaHex string) imageConfig {
	imageConfigPath := getConfigPathForImage(imageShaHex)
	data, err := os.ReadFile(imageConfigPath)
	if err != nil {
		log.Fatalln("get image config failed")
	}
	imgConfig := imageConfig{}
	if err := json.Unmarshal(data, &imgConfig); err != nil {
		log.Fatalln("unmarshal image config failed")
	}
	return imgConfig
}

func deleteImageByHash(imageShaHex string) {
	imgName, tagName := imageExistByHash(imageShaHex)
	if len(imgName) == 0 {
		log.Println("no such image")
		return
	}
	containers, err := getRunningContainers()
	if err != nil {
		log.Fatalln("get running container failed")
	}
	for _, container := range containers {
		if container.image == imgName+":"+tagName {
			log.Fatalf("cat not delete the image used by container %s", container.containerId)
		}
	}
	if err := os.RemoveAll(getGockerImagesPath() + "/" + imageShaHex); err != nil {
		log.Fatalln("remove image dir failed")
	}
	removeImageMetadata(imageShaHex)
}

func removeImageMetadata(imageShaHex string) {
	idb := imagesDB{}
	ientries := imageEntries{}
	parseImagesMetadata(&idb)
	imgName, _ := imageExistByHash(imageShaHex)
	if len(imgName) == 0 {
		log.Fatalln("get image details failed")
	}
	ientries = idb[imgName]
	for tag, hash := range ientries {
		if hash == imgName {
			delete(ientries, tag)
		}
	}
	if len(ientries) == 0 {
		delete(idb, imgName)
	} else {
		idb[imgName] = ientries
	}
	marshalImagesMetadata(idb)
}

func printAvailableImages() {
	idb := imagesDB{}
	parseImagesMetadata(&idb)
	fmt.Printf("IMAGE\t             TAG\t   ID\n")
	for image, entries := range idb {
		fmt.Print(image)
		for tag, hash := range entries {
			fmt.Printf("\t%16s %s\n", tag, hash)
		}
	}
}

func getManifestPathForImage(imageShaHex string) string {
	return getGockerImagesPath() + "/" + imageShaHex + "/manifest.json"
}

func getConfigPathForImage(imageShaHex string) string {
	return getGockerImagesPath() + "/" + imageShaHex + "/" + imageShaHex + ".json"
}

func deleteTempImagePath(imageShaHex string) {
	path := getGockerTempPath() + "/" + imageShaHex
	if err := os.RemoveAll(path); err != nil {
		log.Fatalln("remove temp image dir failed")
	}
}

func untarFile(imageShaHex string) {
	pathDir := getGockerTempPath() + "/" + imageShaHex
	pathTar := pathDir + "/package.tar"
	if err := untar(pathTar, pathDir); err != nil {
		log.Fatalln("untar image tarball failed")
	}
}

func downloadImage(image v1.Image, imageShaHex string, src string) {
	path := getGockerTempPath() + "/" + imageShaHex
	os.Mkdir(path, 0755)
	path += "/package.tar"
	if err := crane.SaveLegacy(image, src, path); err != nil {
		log.Fatalln("save tarball failed")
	}
	log.Printf("download %s success", src)
}

func storeImageMetadata(imgName string, tagName string, imageShaHex string) {
	idb := imagesDB{}
	ientry := imageEntries{}
	parseImagesMetadata(&idb)
	if idb[imgName] != nil {
		ientry = idb[imgName]
	}
	ientry[tagName] = imageShaHex
	idb[imgName] = ientry
	marshalImagesMetadata(idb)
}

func getImageNameAndTag(src string) (string, string) {
	s := strings.Split(src, ":")
	var img, tag string
	if len(s) > 1 {
		img = s[0]
		tag = s[1]
	} else {
		img = s[0]
		tag = "latest"
	}
	return img, tag
}

func imageExistByHash(imageShaHex string) (string, string) {
	idb := imagesDB{}
	parseImagesMetadata(&idb)
	for imgName, entries := range idb {
		for tag, hash := range entries {
			if hash == imageShaHex {
				return imgName, tag
			}
		}
	}
	return "", ""
}

func imageExistByTag(imgName, tagName string) (bool, string) {
	idb := imagesDB{}
	parseImagesMetadata(&idb)
	for img, entries := range idb {
		if img == imgName {
			for tag, imgHash := range entries {
				if tag == tagName {
					return true, imgHash
				}
			}
		}
	}
	return false, ""
}

func parseImagesMetadata(idb *imagesDB) {
	imagesDBPath := getGockerImagesPath() + "/images.json"
	if _, err := os.Stat(imagesDBPath); os.IsNotExist(err) {
		os.WriteFile(imagesDBPath, []byte("{}"), 0644)
	}
	data, err := os.ReadFile(imagesDBPath)
	if err != nil {
		log.Fatalln("read images data failed")
	}
	if err := json.Unmarshal(data, idb); err != nil {
		log.Fatalln("unmarshal images data failed")
	}

}

func marshalImagesMetadata(idb imagesDB) {
	fileBytes, err := json.Marshal(idb)
	if err != nil {
		log.Fatalln("marshal image data failed")
	}
	imagesDBPath := getGockerImagesPath() + "/images.json"
	if err := os.WriteFile(imagesDBPath, fileBytes, 0644); err != nil {
		log.Fatalln("write images metadata failed")
	}
}
