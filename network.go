package main

import (
	"fmt"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	"log"
	"math/rand"
	"net"
)

func isGockerBridgeUp() (bool, error) {
	if links, err := netlink.LinkList(); err != nil {
		log.Printf("get list of links failed: %v", err)
		return false, err
	} else {
		for _, link := range links {
			if link.Type() == "bridge" && link.Attrs().Name == "gocker0" {
				return true, nil
			}
		}
		return false, nil
	}
}

func setupVirtualEthOnHost(containerId string) error {
	veth0 := "veth0_" + containerId[:6]
	veth1 := "veth1_" + containerId[:6]
	linkAttrs := netlink.NewLinkAttrs()
	linkAttrs.Name = veth0
	veth0Struct := &netlink.Veth{
		LinkAttrs:        linkAttrs,
		PeerName:         veth1,
		PeerHardwareAddr: createMACAddress(),
	}
	if err := netlink.LinkAdd(veth0Struct); err != nil {
		return err
	}
	netlink.LinkSetUp(veth0Struct)
	gockerBridge, _ := netlink.LinkByName("gocker0")
	netlink.LinkSetMaster(veth0Struct, gockerBridge)
	return nil
}

func setupNewNetworkNamespace(containerId string) {
	createDirsIfNotExist([]string{getGockerNetNsPath()})
	nsMount := getGockerNetNsPath() + "/" + containerId
	if _, err := unix.Open(nsMount, unix.O_RDONLY|unix.O_CREAT|unix.O_EXCL, 0644); err != nil {
		log.Fatalln("open network mount point failed")
	}

	fd, err := unix.Open("/proc/self/ns/net", unix.O_RDONLY, 0)
	defer unix.Close(fd)
	if err != nil {
		log.Fatalln("open fd failed")
	}
	if err := unix.Unshare(unix.CLONE_NEWNET); err != nil {
		log.Fatalln("unshare system call failed")
	}
	if err := unix.Mount("/proc/self/ns/net", nsMount, "bind", unix.MS_BIND, ""); err != nil {
		log.Fatalln("mount network namespace failed")
	}
	if err := unix.Setns(fd, unix.CLONE_NEWNET); err != nil {
		log.Fatalln("setns system call failed")
	}
}

func createMACAddress() net.HardwareAddr {
	hw := make(net.HardwareAddr, 6)
	hw[0] = 0x02
	hw[1] = 0x42
	rand.Read(hw[2:])
	return hw
}

func setupContainerNetworkInterfaceStep1(containerId string) {
	nsMount := getGockerNetNsPath() + "/" + containerId
	fd, err := unix.Open(nsMount, unix.O_RDONLY, 0)
	defer unix.Close(fd)
	if err != nil {
		log.Fatalln("open fd failed")
	}
	veth1 := "veth1_" + containerId[:6]
	veth1Link, err := netlink.LinkByName(veth1)
	if err != nil {
		log.Fatalln("fetch veth1 failed")
	}
	if err := netlink.LinkSetNsFd(veth1Link, fd); err != nil {
		log.Fatalln("set network namespace for veth1 failed")
	}
}

func setupContainerNetworkInterfaceStep2(containerId string) {
	nsMount := getGockerNetNsPath() + "/" + containerId
	fd, err := unix.Open(nsMount, unix.O_RDONLY, 0)
	defer unix.Close(fd)
	if err != nil {
		log.Fatalln("open network fd failed")
	}
	if err := unix.Setns(fd, unix.CLONE_NEWNET); err != nil {
		log.Fatalln("setns system call failed")
	}

	veth1 := "veth1_" + containerId[:6]
	veth1Link, err := netlink.LinkByName(veth1)
	if err != nil {
		log.Fatalln("fetch veth1 failed")
	}
	addr, _ := netlink.ParseAddr(createIPAddress() + "/16")
	if err := netlink.AddrAdd(veth1Link, addr); err != nil {
		log.Fatalln("assign ip to veth1 failed")
	}
	if err := netlink.LinkSetUp(veth1Link); err != nil {
		log.Fatalln("set up veth1 failed")
	}

	route := netlink.Route{
		Scope:     netlink.SCOPE_UNIVERSE,
		LinkIndex: veth1Link.Attrs().Index,
		Gw:        net.ParseIP("172.29.0.1"),
		Dst:       nil,
	}
	if err := netlink.RouteAdd(&route); err != nil {
		log.Fatalln("add default route failed")
	}
}

func joinContainerNetworkNamespace(containerId string) error {
	nsMount := getGockerNetNsPath() + "/" + containerId
	fd, err := unix.Open(nsMount, unix.O_RDONLY, 0)
	if err != nil {
		return err
	}
	if err := unix.Setns(fd, unix.CLONE_NEWNET); err != nil {
		return err
	}
	return nil
}

func setupLocalInterface() {
	links, _ := netlink.LinkList()
	for _, link := range links {
		if link.Attrs().Name == "lo" {
			loAddr, _ := netlink.ParseAddr("127.0.0.1/32")
			if err := netlink.AddrAdd(link, loAddr); err != nil {
				log.Fatalln("set up local interface failed")
			}
			netlink.LinkSetUp(link)
		}
	}
}

func createIPAddress() string {
	byte1 := rand.Intn(254)
	byte2 := rand.Intn(254)
	return fmt.Sprintf("172.29.%d.%d", byte1, byte2)
}

func setupGockerBridge() error {
	linkAttrs := netlink.NewLinkAttrs()
	linkAttrs.Name = "gocker0"
	gockerBridge := &netlink.Bridge{
		LinkAttrs: linkAttrs,
	}
	if err := netlink.LinkAdd(gockerBridge); err != nil {
		return err
	}
	addr, err := netlink.ParseAddr("172.29.0.1/16")
	if err != nil {
		return err
	}
	netlink.AddrAdd(gockerBridge, addr)
	netlink.LinkSetUp(gockerBridge)
	return nil
}
