package main

import (
  "os"
  "fmt"
  "syscall"
  "os/exec"
  "path/filepath"
  "math/rand"
)

func parent() {
  cmd := exec.Command("/proc/self/exe", append([]string{"child"}, os.Args[2:]...)...)
  cmd.Stdin = os.Stdin
  cmd.Stdout = os.Stdout
  cmd.Stderr = os.Stderr

  // Pass flags to CMD
  cmd.SysProcAttr = &syscall.SysProcAttr{
    Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWUSER | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWNET,
    UidMappings: []syscall.SysProcIDMap{
      {
        ContainerID: 0,
        HostID:      os.Getuid(),
        Size:        1,
      },
    },
    GidMappings: []syscall.SysProcIDMap{
      {
        ContainerID: 0,
        HostID:      os.Getgid(),
        Size:        1,
      },
    },
  }

  if err := cmd.Start(); err != nil {
    fmt.Println("ERROR", err)
    os.Exit(1)
  }

  must(createBridge())
  must(createVethPair(cmd.Process.Pid))
  cmd.Wait()
}

func child() {
  cmd := exec.Command(os.Args[2], os.Args[3:]...)
  cmd.Stdin = os.Stdin
  cmd.Stdout = os.Stdout
  cmd.Stderr = os.Stderr

  //Initialize NS
  must(syscall.Sethostname([]byte("fjfc")))
  must(pivotRoot("./alpine"))
  must(syscall.Mount("proc", "/proc", "proc", syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV, ""))
  must(syscall.Mount("tmp", "/tmp", "tmpfs", syscall.MS_NOSUID | syscall.MS_STRICTATIME, "mode=755"))

  // Network
  lnk, err := waitForIface()
  if err != nil {
    panic(err)
  }

  ifaceIP := fmt.Sprintf(ipTmpl, rand.Intn(253)+2)
  if err := setupIface(lnk, ifaceIP); err != nil {
    panic(err)
  }

  if err := cmd.Run(); err != nil {
    fmt.Println("ERROR", err)
    os.Exit(1)
  }
  syscall.Unmount("/proc",0)
  syscall.Unmount("/tmp",0)
}

func pivotRoot(root string) error {
  // we need this to satisfy restriction:
  // "new_root and put_old must not be on the same filesystem as the current root"
  if err := syscall.Mount(root, root, "bind", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
    return fmt.Errorf("Mount rootfs to itself error: %v", err)
  }
  // create rootfs/.pivot_root as path for old_root
  pivotDir := filepath.Join(root, ".pivot_root")
  if err := os.Mkdir(pivotDir, 0777); err != nil {
    return err
  }
  // pivot_root to rootfs, now old_root is mounted in rootfs/.pivot_root
  // mounts from it still can be seen in `mount`
  if err := syscall.PivotRoot(root, pivotDir); err != nil {
    return fmt.Errorf("pivot_root %v", err)
  }
  // change working directory to /
  // it is recommendation from man-page
  if err := syscall.Chdir("/"); err != nil {
    return fmt.Errorf("chdir / %v", err)
  }
  // path to pivot root now changed, update
  pivotDir = filepath.Join("/", ".pivot_root")
  // umount rootfs/.pivot_root(which is now /.pivot_root) with all submounts
  // now we have only mounts that we mounted ourselves in `mount`
  if err := syscall.Unmount(pivotDir, syscall.MNT_DETACH); err != nil {
    return fmt.Errorf("unmount pivot_root dir %v", err)
  }
  // remove temporary directory
  return os.Remove(pivotDir)
}

