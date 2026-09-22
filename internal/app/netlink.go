//go:build linux

package app

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

const (
	sockDiagByFamily = 20
	inetDiagReqSize  = 56
	inetDiagMsgSize  = 72
	tcpListenState   = 10
)

type netlinkSource struct{}

func (netlinkSource) Sockets(ctx context.Context) ([]Port, error) {
	var rows []Port
	for _, query := range []struct{ family, protocol byte }{{unix.AF_INET, unix.IPPROTO_TCP}, {unix.AF_INET6, unix.IPPROTO_TCP}, {unix.AF_INET, unix.IPPROTO_UDP}, {unix.AF_INET6, unix.IPPROTO_UDP}} {
		found, err := queryInetDiag(ctx, query.family, query.protocol)
		if err != nil {
			// Restricted containers can block socket diagnostics while leaving procfs readable.
			return procNetSockets("/proc")
		}
		rows = append(rows, found...)
	}
	return rows, nil
}

func procNetSockets(procRoot string) ([]Port, error) {
	var rows []Port
	for _, table := range []struct {
		file, protocol string
		ipv6           bool
	}{{"tcp", "TCP", false}, {"tcp6", "TCP", true}, {"udp", "UDP", false}, {"udp6", "UDP", true}} {
		file, err := os.Open(filepath.Join(procRoot, "net", table.file))
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) < 10 || fields[0] == "sl" {
				continue
			}
			if table.protocol == "TCP" && fields[3] != "0A" {
				continue
			}
			if table.protocol == "UDP" && fields[3] != "07" {
				continue
			}
			address, portHex, ok := strings.Cut(fields[1], ":")
			if !ok {
				continue
			}
			port64, err := strconv.ParseUint(portHex, 16, 16)
			if err != nil {
				continue
			}
			inode64, err := strconv.ParseUint(fields[9], 10, 32)
			if err != nil {
				continue
			}
			ip, err := decodeProcIP(address, table.ipv6)
			if err != nil {
				continue
			}
			rows = append(rows, Port{Protocol: table.protocol, IP: ip.String(), Port: int(port64), inode: uint32(inode64)})
		}
		scanErr := scanner.Err()
		file.Close()
		if scanErr != nil {
			return nil, fmt.Errorf("read %s: %w", file.Name(), scanErr)
		}
	}
	return rows, nil
}

func decodeProcIP(value string, ipv6 bool) (net.IP, error) {
	data, err := hex.DecodeString(value)
	if err != nil {
		return nil, err
	}
	want := 4
	if ipv6 {
		want = 16
	}
	if len(data) != want {
		return nil, fmt.Errorf("invalid address length")
	}
	for offset := 0; offset < len(data); offset += 4 {
		data[offset], data[offset+3] = data[offset+3], data[offset]
		data[offset+1], data[offset+2] = data[offset+2], data[offset+1]
	}
	return net.IP(data), nil
}

func queryInetDiag(ctx context.Context, family, protocol byte) ([]Port, error) {
	fd, err := unix.Socket(unix.AF_NETLINK, unix.SOCK_RAW|unix.SOCK_CLOEXEC, unix.NETLINK_SOCK_DIAG)
	if err != nil {
		return nil, fmt.Errorf("open socket diagnostics: %w", err)
	}
	defer unix.Close(fd)
	if err := unix.Bind(fd, &unix.SockaddrNetlink{Family: unix.AF_NETLINK}); err != nil {
		return nil, err
	}
	seq := uint32(1)
	req := make([]byte, unix.NLMSG_HDRLEN+inetDiagReqSize)
	native := binary.NativeEndian
	native.PutUint32(req[0:4], uint32(len(req)))
	native.PutUint16(req[4:6], sockDiagByFamily)
	native.PutUint16(req[6:8], unix.NLM_F_REQUEST|unix.NLM_F_DUMP)
	native.PutUint32(req[8:12], seq)
	req[16], req[17] = family, protocol
	states := uint32(0xffffffff)
	if protocol == unix.IPPROTO_TCP {
		states = 1 << tcpListenState
	}
	native.PutUint32(req[20:24], states)
	for i := 48; i < 56; i++ {
		req[16+i] = 0xff
	}
	if err := unix.Sendto(fd, req, 0, &unix.SockaddrNetlink{Family: unix.AF_NETLINK}); err != nil {
		return nil, err
	}
	var rows []Port
	buffer := make([]byte, 64*1024)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		n, _, err := unix.Recvfrom(fd, buffer, 0)
		if err != nil {
			return nil, fmt.Errorf("read socket diagnostics: %w", err)
		}
		messages, err := syscall.ParseNetlinkMessage(buffer[:n])
		if err != nil {
			return nil, err
		}
		for _, message := range messages {
			if message.Header.Seq != seq {
				continue
			}
			switch message.Header.Type {
			case unix.NLMSG_DONE:
				return rows, nil
			case unix.NLMSG_ERROR:
				return nil, fmt.Errorf("socket diagnostics returned an error")
			case sockDiagByFamily:
				if len(message.Data) < inetDiagMsgSize {
					continue
				}
				if protocol == unix.IPPROTO_UDP && binary.BigEndian.Uint16(message.Data[6:8]) != 0 {
					continue
				}
				rows = append(rows, decodeInetDiag(message.Data, protocol))
			}
		}
	}
}

func decodeInetDiag(data []byte, protocol byte) Port {
	var ip net.IP
	if data[0] == unix.AF_INET {
		ip = net.IP(data[8:12])
	} else {
		ip = net.IP(data[8:24])
	}
	proto := "TCP"
	if protocol == unix.IPPROTO_UDP {
		proto = "UDP"
	}
	return Port{Protocol: proto, IP: ip.String(), Port: int(binary.BigEndian.Uint16(data[4:6])), inode: binary.NativeEndian.Uint32(data[68:72])}
}
