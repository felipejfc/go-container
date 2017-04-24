package main

import (
  "net"
  "strings"
  "fmt"
  "github.com/vishvananda/netlink"
  "time"
  "math/rand"
)

const (
  bridgeName = "fjfc0"
  ipAddr     = "10.100.55.1/24"
  vethPrefix = "fj"
  ipTmpl     = "10.100.55.%d/24"
)

func createBridge() error {
  // try to get bridge by name, if it already exists then just exit
  _, err := net.InterfaceByName(bridgeName)
  if err == nil {
    return nil
  }
  if !strings.Contains(err.Error(), "no such network interface") {
    return err
  }
  // create *netlink.Bridge object
  la := netlink.NewLinkAttrs()
  la.Name = bridgeName
  br := &netlink.Bridge{la}
  if err := netlink.LinkAdd(br); err != nil {
    return fmt.Errorf("bridge creation: %v", err)
  }
  // set up ip addres for bridge
  addr, err := netlink.ParseAddr(ipAddr)
  if err != nil {
    return fmt.Errorf("parse address %s: %v", ipAddr, err)
  }
  if err := netlink.AddrAdd(br, addr); err != nil {
    return fmt.Errorf("add address %v to bridge: %v", addr, err)
  }
  // sets up bridge ( ip link set dev unc0 up )
  if err := netlink.LinkSetUp(br); err != nil {
    return err
  }
  return nil
}

func createVethPair(pid int) error {
  // get bridge to set as master for one side of veth-pair
  br, err := netlink.LinkByName(bridgeName)
  if err != nil {
    return err
  }
  // generate names for interfaces
  x1, x2 := rand.Intn(10000), rand.Intn(10000)
  parentName := fmt.Sprintf("%s%d", vethPrefix, x1)
  peerName := fmt.Sprintf("%s%d", vethPrefix, x2)
  // create *netlink.Veth
  la := netlink.NewLinkAttrs()
  la.Name = parentName
  la.MasterIndex = br.Attrs().Index
  vp := &netlink.Veth{LinkAttrs: la, PeerName: peerName}
  if err := netlink.LinkAdd(vp); err != nil {
    return fmt.Errorf("veth pair creation %s <-> %s: %v", parentName, peerName, err)
  }
  // get peer by name to put it to namespace
  peer, err := netlink.LinkByName(peerName)
  if err != nil {
    return fmt.Errorf("get peer interface: %v", err)
  }
  // put peer side to network namespace of specified PID
  if err := netlink.LinkSetNsPid(peer, pid); err != nil {
    return fmt.Errorf("move peer to ns of %d: %v", pid, err)
  }
  if err := netlink.LinkSetUp(vp); err != nil {
    return err
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
  err =  netlink.AddrAdd(link, addr)
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
