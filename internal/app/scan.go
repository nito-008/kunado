package app

import (
	"context"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

type Port struct {
	Protocol, IP, Process, Command, CWD, User string
	Port, PID                                 int
	Container, ComposeProject, ComposeService string
	ContainerPort                             string
	inode                                     uint32
}

type socketSource interface {
	Sockets(context.Context) ([]Port, error)
}
type containerSource interface {
	Containers(context.Context) (map[int]containerInfo, error)
}

type Scanner struct {
	sockets    socketSource
	containers containerSource
	procRoot   string
}

func NewScanner() *Scanner {
	return &Scanner{sockets: netlinkSource{}, containers: newDockerSource(), procRoot: "/proc"}
}

func (s *Scanner) Scan(ctx context.Context) ([]Port, error) {
	rows, err := s.sockets.Sockets(ctx)
	if err != nil {
		return nil, err
	}
	owners := socketOwners(s.procRoot, rows)
	containerByPort, _ := s.containers.Containers(ctx)
	for i := range rows {
		if owner, ok := owners[rows[i].inode]; ok {
			rows[i].PID, rows[i].Process = owner.pid, owner.process
		}
		if rows[i].Process == "" {
			rows[i].Process = "-"
		}
		if c, ok := containerByPort[rows[i].Port]; ok && (rows[i].Process == "docker-proxy" || rows[i].Process == "-") {
			rows[i].Container, rows[i].ComposeProject, rows[i].ComposeService = c.Name, c.Project, c.Service
			rows[i].ContainerPort, rows[i].CWD = c.ContainerPort, c.WorkDir
			rows[i].Process = "docker:" + c.Name
		}
		if rows[i].PID > 0 {
			enrichProcess(&rows[i], s.procRoot)
		}
	}
	sortPorts(rows, sortPort, true)
	return rows, nil
}

type processOwner struct {
	pid     int
	process string
}

func socketOwners(procRoot string, sockets []Port) map[uint32]processOwner {
	wanted := make(map[uint32]struct{}, len(sockets))
	for _, socket := range sockets {
		wanted[socket.inode] = struct{}{}
	}
	owners := make(map[uint32]processOwner)
	entries, _ := os.ReadDir(procRoot)
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || !entry.IsDir() {
			continue
		}
		fds, err := os.ReadDir(filepath.Join(procRoot, entry.Name(), "fd"))
		if err != nil {
			continue
		}
		for _, fd := range fds {
			target, err := os.Readlink(filepath.Join(procRoot, entry.Name(), "fd", fd.Name()))
			if err != nil || !strings.HasPrefix(target, "socket:[") {
				continue
			}
			inode64, err := strconv.ParseUint(strings.TrimSuffix(strings.TrimPrefix(target, "socket:["), "]"), 10, 32)
			inode := uint32(inode64)
			if err != nil {
				continue
			}
			if _, ok := wanted[inode]; !ok {
				continue
			}
			name, _ := os.ReadFile(filepath.Join(procRoot, entry.Name(), "comm"))
			owners[inode] = processOwner{pid: pid, process: strings.TrimSpace(string(name))}
		}
	}
	return owners
}

func enrichProcess(row *Port, procRoot string) {
	base := filepath.Join(procRoot, strconv.Itoa(row.PID))
	if value, err := os.Readlink(filepath.Join(base, "cwd")); err == nil && row.CWD == "" {
		row.CWD = value
	}
	if value, err := os.ReadFile(filepath.Join(base, "cmdline")); err == nil {
		row.Command = strings.TrimSpace(strings.ReplaceAll(string(value), "\x00", " "))
	}
	if info, err := os.Stat(base); err == nil {
		if stat, ok := info.Sys().(*syscall.Stat_t); ok {
			if account, err := user.LookupId(strconv.FormatUint(uint64(stat.Uid), 10)); err == nil {
				row.User = account.Username
			}
		}
	}
}

type containerInfo struct{ Name, Project, Service, WorkDir, ContainerPort string }
