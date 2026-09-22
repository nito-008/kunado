package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type dockerSource struct{ client *http.Client }

func newDockerSource() dockerSource {
	dialer := net.Dialer{Timeout: 250 * time.Millisecond}
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return dialer.DialContext(ctx, "unix", "/var/run/docker.sock")
	}}
	return dockerSource{client: &http.Client{Transport: transport, Timeout: time.Second}}
}

func (d dockerSource) Containers(ctx context.Context) (map[int]containerInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker/containers/json", nil)
	if err != nil {
		return nil, err
	}
	response, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Docker API: %s", response.Status)
	}
	var items []struct {
		Names  []string          `json:"Names"`
		Labels map[string]string `json:"Labels"`
		Ports  []struct {
			PrivatePort, PublicPort uint16
			Type, IP                string
		} `json:"Ports"`
	}
	if err := json.NewDecoder(response.Body).Decode(&items); err != nil {
		return nil, err
	}
	result := make(map[int]containerInfo)
	for _, item := range items {
		name := ""
		if len(item.Names) > 0 {
			name = strings.TrimPrefix(item.Names[0], "/")
		}
		for _, port := range item.Ports {
			if port.PublicPort == 0 {
				continue
			}
			result[int(port.PublicPort)] = containerInfo{Name: name, Project: item.Labels["com.docker.compose.project"], Service: item.Labels["com.docker.compose.service"], WorkDir: item.Labels["com.docker.compose.project.working_dir"], ContainerPort: strconv.Itoa(int(port.PrivatePort))}
		}
	}
	return result, nil
}
