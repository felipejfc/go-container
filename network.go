package main

import (
	"fmt"
	"github.com/vishvananda/netlink"
	"os/exec"
	"strconv"
	"time"
)

const (
	ipTmpl     = "10.100.42.%d/24"
	suidBridge = "net/bridge"
)

func putIface(pid int) error {
	cmd := exec.Command(suidBridge, strconv.Itoa(pid))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("bridge: out: %s, err: %v", out, err)
	}
	return nil
}

func setupIface(link netlink.Link, ip string) error {
	// up loopback
	lo, err := netlink.LinkByName("lo")
	if err != nil {
		return fmt.Errorf("lo interface: %v", err)
	}
	if err := netlink.LinkSetUp(lo); err != nil {
		return fmt.Errorf("up veth: %v", err)
	}
	addr, err := netlink.ParseAddr(ip)
	if err != nil {
		return fmt.Errorf("parse IP: %v", err)
	}
	err = netlink.AddrAdd(link, addr)
	if err != nil {
		return fmt.Errorf("addr add: %v", err)
	}
	return netlink.LinkSetUp(link)
}

func waitForIface() (netlink.Link, error) {
	start := time.Now()
	for {
		fmt.Printf(".")
		if time.Since(start) > 5*time.Second {
			fmt.Printf("\n")
			return nil, fmt.Errorf("failed to find veth interface in 5 seconds")
		}
		// get list of all interfaces
		lst, err := netlink.LinkList()
		if err != nil {
			fmt.Printf("\n")
			return nil, err
		}
		for _, l := range lst {
			// if we found "veth" interface - it's time to continue setup
			if l.Type() == "veth" {
				fmt.Printf("\n")
				return l, nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
}
