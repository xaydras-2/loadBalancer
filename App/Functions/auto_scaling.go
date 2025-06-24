package functions

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"

	"github.com/xaydras-2/loadBalancer/App/structers"
)

// CreateReplicas spins up one new container instance of your API, listening
// on the given hostPort, and returns a Backend pointing to it.
func CreateReplicas(imageName string, containerPort, hostPort string) (string, *structers.Backend, error) {
	ctx := context.Background()

	// 1. Create Docker client
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return "", nil, fmt.Errorf("docker client init: %w", err)
	}
	defer cli.Close()

	// 2. Pull image if missing
	imgs, err := cli.ImageList(ctx, image.ListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("reference", imageName)),
	})
	if err != nil {
		return "", nil, fmt.Errorf("image list: %w", err)
	}
	if len(imgs) == 0 {
		out, err := cli.ImagePull(ctx, imageName, image.PullOptions{})
		if err != nil {
			return "", nil, fmt.Errorf("image pull: %w", err)
		}
		io.Copy(os.Stdout, out)
		out.Close()
	}

	// 3. Set up port mapping
	portKey := nat.Port(containerPort + "/tcp")
	exposed := nat.PortSet{portKey: struct{}{}}
	bindings := nat.PortMap{portKey: []nat.PortBinding{
		{HostIP: "0.0.0.0", HostPort: hostPort},
	}}

	// 4. Create the container
	resp, err := cli.ContainerCreate(
		ctx,
		&container.Config{
			Image:        imageName,
			Cmd: []string{ "--port", containerPort},
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
		return "", nil, fmt.Errorf("container create: %w", err)
	}

	// 5. Start the container
	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", nil, fmt.Errorf("container start: %w", err)
	}

	containerID := resp.ID

	// 6. Build the Backend struct pointing at our new instance
	urlStr := fmt.Sprintf("http://localhost:%s", hostPort)
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return "", nil, fmt.Errorf("invalid URL %q: %w", urlStr, err)
	}
	backend := &structers.Backend{
		URL:   parsed,
		Alive: true, // default to true, health will be checked by the LoadBalancer
	}

	return containerID, backend, nil
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
