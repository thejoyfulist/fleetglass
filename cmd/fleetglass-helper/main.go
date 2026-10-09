package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/user"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/config"
	"github.com/thejoyfulist/fleetglass/internal/helper"
	"github.com/thejoyfulist/fleetglass/internal/projects"
	"github.com/thejoyfulist/fleetglass/internal/version"
)

func main() {
	path := flag.String("config", "", "JSON config path")
	group := flag.String("group", "fleetglass", "socket reader group")
	showVersion := flag.Bool("version", false, "show version")
	flag.Parse()
	if *showVersion {
		fmt.Println(version.String())
		return
	}
	c, e := config.Load(*path)
	if e != nil {
		log.Fatal(e)
	}
	h := helper.Handler{Devices: c.SMARTDevices, DockerSockets: c.DockerSockets, SysRoot: c.SysRoot, ProcRoot: c.ProcRoot, AgentCommands: c.AgentCommands, BackupSets: c.BackupSets, StatusFiles: c.StatusFiles, LogMarkers: c.LogMarkers, StorageRoots: c.StorageRoots, ProjectRoots: c.ProjectRoots, WorktreeRoots: c.WorktreeRoots, ProjectCollector: &projects.Collector{}, ProjectMu: &sync.Mutex{}}
	if e := h.Validate(); e != nil {
		log.Fatal(e)
	}
	g, e := user.LookupGroup(*group)
	if e != nil {
		log.Fatal(e)
	}
	gid, e := strconv.Atoi(g.Gid)
	if e != nil {
		log.Fatal(e)
	}
	if e := os.Remove(c.HelperSocket); e != nil && !os.IsNotExist(e) {
		log.Fatal(e)
	}
	l, e := protectedListen(c.HelperSocket)
	if e != nil {
		log.Fatal(e)
	}
	defer os.Remove(c.HelperSocket)
	defer l.Close()
	if e := os.Chown(c.HelperSocket, -1, gid); e != nil {
		log.Fatal(e)
	}
	if e := os.Chmod(c.HelperSocket, 0660); e != nil {
		log.Fatal(e)
	}
	server := &http.Server{Handler: h, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 1 << 16}
	log.Fatal(server.Serve(l))
}

// protectedListen restricts the socket from the moment it appears in the filesystem.
func protectedListen(path string) (net.Listener, error) {
	oldMask := syscall.Umask(0177)
	defer syscall.Umask(oldMask)
	return net.Listen("unix", path)
}
