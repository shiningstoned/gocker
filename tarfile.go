package main

import (
	"archive/tar"
	"io"
	"log"
	"os"
	"path/filepath"
)

func untar(tarball, target string) error {
	hardlink := make(map[string]string)
	reader, err := os.Open(tarball)
	if err != nil {
		return err
	}
	defer reader.Close()

	tarReader := tar.NewReader(reader)

	for {
		header, err := tarReader.Next()
		if err != nil {
			if err == io.EOF {
				break
			} else {
				return err
			}
		}

		path := filepath.Join(target, header.Name)
		info := header.FileInfo()

		switch header.Typeflag {
		case tar.TypeDir:
			if err = os.MkdirAll(path, info.Mode()); err != nil {
				return err
			}
			continue
		case tar.TypeLink:
			linkPath := filepath.Join(target, header.Linkname)
			linkPath2 := filepath.Join(target, header.Name)
			hardlink[linkPath2] = linkPath
			continue
		case tar.TypeSymlink:
			linkPath := filepath.Join(target, header.Name)
			if err := os.Symlink(header.Linkname, linkPath); err != nil {
				if os.IsExist(err) {
					continue
				}
				return err
			}
			continue
		case tar.TypeReg:
			if _, err := os.Stat(filepath.Dir(path)); os.IsNotExist(err) {
				os.MkdirAll(filepath.Dir(path), 0755)
			}
			file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
			if os.IsExist(err) {
				continue
			}
			if err != nil {
				return err
			}
			_, err = io.Copy(file, tarReader)
			file.Close()
			if err != nil {
				return err
			}
		default:
			log.Println("unknown file type")
		}
	}

	for k, v := range hardlink {
		if err := os.Link(v, k); err != nil {
			return err
		}
	}

	return nil
}
