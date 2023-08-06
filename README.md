# gocker
a tiny docker

## capabilities
* Run a process in a container
  ` gocker run <--mem> <--pids> <--cpus> <image:tag> [cmd] `
* List running containers
  ` gocker ps `
* List local images
  ` gocker images `
* Execute a process in a running container
  ` gocker exec <container-ID> [cmd] `
* Remove a local image
  ` gocker rmi <image-ID> `

## container isolation
Containers created with Gocker get the following namespaces of their own:
* File System
* PID
* UTS
* Network
* IPC
* Mount

## Example
```
go build -o gocker
sudo ./gocker run alpine /bin/sh
2023/08/06 13:16:42 container Id a87d9812016d
2023/08/06 13:16:42 downloading metadata for alpine:latest, please wait
2023/08/06 13:16:45 checking if image exists under another name
2023/08/06 13:16:45 image do not exist, downloading image...
2023/08/06 13:16:50 download alpine success
2023/08/06 13:16:50 image to overlay mount c1aabb73d233
/ # exit
2023/08/06 13:17:03 container done

sudo ./gocker images
IMAGE                TAG           ID
alpine            latest c1aabb73d233
```
