package functions

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"time"

	"encoding/json"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"

	"github.com/xaydras-2/loadBalancer/App/structers"
)

// CreateReplicas spins up one new container instance of your API, listening
// on the given hostPort, and returns a Backend pointing to it.
func CreateReplicas(imageName string, containerPort string) (*structers.Backend, error) {
	ctx := context.Background()

	// Create Docker client
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("docker client init: %w", err)
	}
	defer cli.Close()

	// Pull image if missing
	imgs, err := cli.ImageList(ctx, image.ListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("reference", imageName)),
	})
	if err != nil {
		return nil, fmt.Errorf("image list: %w", err)
	}
	if len(imgs) == 0 {
		out, err := cli.ImagePull(ctx, imageName, image.PullOptions{})
		if err != nil {
			return nil, fmt.Errorf("image pull: %w", err)
		}
		io.Copy(os.Stdout, out)
		out.Close()
	}

	// Set up port mapping
	portKey := nat.Port(containerPort + "/tcp")
	exposed := nat.PortSet{portKey: struct{}{}}
	bindings := nat.PortMap{portKey: []nat.PortBinding{
		{HostIP: "0.0.0.0", HostPort: ""},
	}}

	// Create the container
	resp, err := cli.ContainerCreate(
		ctx,
		&container.Config{
			Image:        imageName,
			Cmd:          []string{"--port", containerPort},
			ExposedPorts: exposed,
		},
		&container.HostConfig{
			PortBindings: bindings,
		},
		nil, // *network.NetworkingConfig
		nil, // *specs.Platform
		"",  // container name
	)
	if err != nil {
		return nil, fmt.Errorf("container create: %w", err)
	}

	// Start the container
	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return nil, fmt.Errorf("container start: %w", err)
	}

	containerID := resp.ID

	portExtKey := nat.Port(containerPort + "/tcp")
	var hostPort string

	const (
		maxRetries = 10
		sleepMs    = 100
	)
	for i := 0; i < maxRetries; i++ {
		insp, err := cli.ContainerInspect(ctx, resp.ID)
		if err != nil {
			return nil, fmt.Errorf("inspect container (attempt %d): %w", i+1, err)
		}
		bindings := insp.NetworkSettings.Ports[portExtKey]
		if len(bindings) > 0 && bindings[0].HostPort != "" {
			hostPort = bindings[0].HostPort
			break
		}
		time.Sleep(sleepMs * time.Millisecond)
	}

	// Build the Backend struct pointing at our new instance
	urlStr := fmt.Sprintf("http://localhost:%s", hostPort)
	parsed, err := url.Parse(urlStr)

	if err != nil {
		return nil, fmt.Errorf("invalid URL %q: %w", urlStr, err)
	}

	backend := &structers.Backend{
		URL:         parsed,
		Alive:       true, // default to true, health will be checked by the LoadBalancer
		ContainerID: containerID,
	}

	return backend, nil
}

func CloseReplicas(containerID string) (string, error) {
	ctx := context.Background()

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return "", fmt.Errorf("docker client init: %w", err)
	}
	defer cli.Close()

	// 1. Stop the container
	if err := cli.ContainerStop(ctx, containerID, container.StopOptions{}); err != nil {
		return "", fmt.Errorf("failed to stop container %q: %w", containerID, err)
	}

	// 2. Remove the container after stopping it
	if err := cli.ContainerRemove(ctx, containerID, container.RemoveOptions{}); err != nil {
		return "", fmt.Errorf("failed to remove container %q: %w", containerID, err)
	}

	return fmt.Sprintf("Container %q stopped and removed successfully", containerID), nil
}

// gets the cpu and memory of a given container id
func GetInfoAboutReplica(containerID string) (*structers.ReplicaStats, error) {
	ctx := context.Background()

	// initialize Docker client
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("docker client init: %w", err)
	}
	defer cli.Close()

	// fetch one-shot stats (non-streaming)
	statsRes, err := cli.ContainerStats(ctx, containerID, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}
	defer statsRes.Body.Close()

	// decode into StatsJSON
	var s structers.StatsJSON
	if err := json.NewDecoder(statsRes.Body).Decode(&s); err != nil {
		return nil, fmt.Errorf("failed to decode stats: %w", err)
	}

	// calculate CPU %
	cpuDelta := float64(s.CPUStats.CPUUsage.TotalUsage - s.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(s.CPUStats.SystemUsage - s.PreCPUStats.SystemUsage)
	cpuPercent := 0.0
	if systemDelta > 0.0 && cpuDelta > 0.0 {
		cpuCount := float64(len(s.CPUStats.CPUUsage.PercpuUsage))
		cpuPercent = (cpuDelta / systemDelta) * cpuCount * 100.0
	}

	// calculate Memory %
	// subtract cache to get the real cache
	memUsage := s.MemoryStats.Usage - s.MemoryStats.Stats["cache"]
	memLimit := s.MemoryStats.Limit
	memPercent := 0.0
	if memLimit > 0 {
		memPercent = (float64(memUsage) / float64(memLimit)) * 100.0
	}

	return &structers.ReplicaStats{
		CPUPercent:    cpuPercent,
		MemoryUsage:   memUsage,
		MemoryLimit:   memLimit,
		MemoryPercent: memPercent,
	}, nil
}
