// Package functions implements core logic for active monitoring, load balancing,
// and auto-scaling of back-end services.
package functions

import (
	"container/heap"
	"context"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"encoding/json"

	composeLoader "github.com/compose-spec/compose-go/loader"
	composeTypes "github.com/compose-spec/compose-go/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"

	"github.com/xaydras-2/loadBalancer/App/config"
	"github.com/xaydras-2/loadBalancer/App/structers"
)

// CreateReplicas spins up one new container instance of your API, listening
// on the given hostPort, and returns a Backend pointing to it.
func CreateReplicas(imageName string, containerPort string, networkName string) (*structers.Backend, error) {
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

	nextIndex, err := NextAPISuffix(cli, ctx, "api", config.ParentName)
	if err != nil {
		log.Fatalf("could not compute next suffix: %v", err)
	}

	containerName := fmt.Sprintf("%s-%d", config.ParentName, nextIndex)

	// Create the container
	resp, err := cli.ContainerCreate(
		ctx,
		&container.Config{
			Image:        imageName,
			Cmd:          []string{"--port", containerPort},
			ExposedPorts: exposed,
			Env: []string{
				"DB_HOST=postgres_db",
				"DB_PORT=5432",
				"DB_USER=postgres",
				"DB_PASSWORD=postgres2025",
				"DB_NAME=test_lb",
			},
			Labels: map[string]string{
				"com.docker.compose.project": "api",
				"com.docker.compose.service": config.SvcTemp.Name,
			},
		},
		&container.HostConfig{
			PortBindings: bindings,
		},
		&network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				networkName: {},
			},
		},
		nil,           // *specs.Platform
		containerName, // container name
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
		StartTime:   time.Now(),
	}

	return backend, nil
}

// NextAPISuffix it extract the name of the container, and gets the number of the last created one.
// Note: it needs the container to follow this formate <parentName>-<N> (Docker compose/ microservice)
func NextAPISuffix(cli *client.Client, ctx context.Context, serviceName, parentName string) (int, error) {
	// 1) List all running containers with label com.docker.compose.service=api
	args := filters.NewArgs(
		filters.Arg("label", "com.docker.compose.service="+serviceName),
	)
	listOpts := container.ListOptions{All: true, Filters: args}
	containers, err := cli.ContainerList(ctx, listOpts)
	if err != nil {
		return 0, fmt.Errorf("listing api containers: %w", err)
	}

	// 2) Extract the numeric suffix out of each container’s Name
	//    Expecting container names like "<parentName>-<N>"
	re := regexp.MustCompile(fmt.Sprintf(`^%s-(\d+)$`, regexp.QuoteMeta(parentName)))
	var nums []int
	for _, c := range containers {
		for _, name := range c.Names {
			// Docker returns names prefixed with '/', so strip it off
			n := strings.TrimPrefix(name, "/")
			if matches := re.FindStringSubmatch(n); matches != nil {
				if i, err := strconv.Atoi(matches[1]); err == nil {
					nums = append(nums, i)
				}
			}
		}
	}

	// 3) Find the max index and return max+1 (or 1 if none found)
	next := 1
	if len(nums) > 0 {
		sort.Ints(nums)
		next = nums[len(nums)-1] + 1
	}
	return next, nil
}

// CloseReplicas closes the replica with the given id, it first stop the container and then shut it down
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

// CallContainers loads the containers of the returned project by loadComposeFile.
// it creates one set of db, and n sets of the api.
func CallContainers() {
	project, err := loadComposeFile(config.DockerComposePath)
	if err != nil {
		log.Fatalf("could not load compose: %v", err)
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatalf("docker client: %v", err)
	}

	defer cli.Close()
	ctx := context.Background()

	// Create networks first
	if err := createNetworks(cli, ctx, project); err != nil {
		log.Fatalf("failed to create networks: %v", err)
	}

	// We take the first network, the loop is needed because Networks are a map string, Fascinating.
	var primaryNetwork string
	for networkName := range project.Networks {
		primaryNetwork = networkName
		config.NetworkName = primaryNetwork
		break
	}

	for _, svc := range project.Services {

		switch svc.Name {

		case "postgres":
			// Ensure exactly 1 replica of db
			if err := ensureDB(cli, ctx, svc, primaryNetwork); err != nil {
				log.Fatalf("db error: %v", err)
			}

		case "api":
			// ! this will be removed, since the using of svcTemp doesn't, for now, hold any meanings, the container name should be the name of the parent and the number of it
			// make the svc be hold by the SvcTemp for it to be used in create replicas
			config.SvcTemp = svc

			for i := 0; i < config.InitialReplicas; i++ {
				backend, err := CreateReplicas(svc.Image, config.ContainerPort, primaryNetwork)
				if err != nil {
					log.Fatalf("create api replica: %v", err)
				}
				fmt.Printf("started API backend: %+v\n", backend)
				// add to the heap
				config.BackendsMu.Lock()
				heap.Push(&config.Backends, backend)
				config.BackendsMu.Unlock()
			}

		default:
			log.Printf("An undefined service case has been detected: %v", svc.Name)
		}
	}
}

// ensureDB makes sure there is exactly one container for the db service
func ensureDB(cli *client.Client, ctx context.Context, svc composeTypes.ServiceConfig, networkName string) error {
	filters := filters.NewArgs(
		filters.Arg("label", "com.docker.compose.service="+svc.Name),
		filters.Arg("status", "running"),
	)
	existing, err := cli.ContainerList(ctx, container.ListOptions{
		All:     false,
		Filters: filters,
	})
	if err != nil {
		return fmt.Errorf("list db containers: %w", err)
	}
	if len(existing) > 0 {
		// already running one—nothing to do
		return nil
	}

	// otherwise, create & start one
	portDef := svc.Ports[0]
	portKey := nat.Port(fmt.Sprintf("%d/tcp", portDef.Target))
	binds := nat.PortMap{
		portKey: []nat.PortBinding{{HostPort: portDef.Published}},
	}

	// Get environment variables from service config
	env := getServiceEnvironment(svc)

	// Set default environment variables if not present
	if len(env) == 0 {
		env = []string{
			"POSTGRES_PASSWORD=postgres2025",
			"POSTGRES_DB=test_lb",
			"POSTGRES_USER=postgres",
		}
	}

	resp, err := cli.ContainerCreate(ctx,
		&container.Config{
			Image:        svc.Image,
			ExposedPorts: nat.PortSet{portKey: struct{}{}},
			Env:          env,
			Labels: map[string]string{
				"com.docker.compose.project": "api",
				"com.docker.compose.service": svc.Name,
			},
		},
		&container.HostConfig{
			PortBindings: binds,
		},
		&network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				networkName: {},
			},
		},
		nil,
		svc.ContainerName,
	)
	if err != nil {
		return fmt.Errorf("create db container: %w", err)
	}
	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("start db container: %w", err)
	}

	log.Printf("Created and started database container: %s", resp.ID)
	return nil
}

// loadComposeFile is used when we have a docker compose file, it returns the set of loaded composer
func loadComposeFile(path string) (*composeTypes.Project, error) {
	configFiles := []composeTypes.ConfigFile{
		{
			Filename: path,
		},
	}

	configDetails := composeTypes.ConfigDetails{
		ConfigFiles: configFiles,
		WorkingDir:  filepath.Dir(path),
	}

	project, err := composeLoader.Load(configDetails)
	if err != nil {
		return nil, fmt.Errorf("load compose project: %w", err)
	}

	return project, nil
}

// createNetworks creates all networks defined in the compose file
func createNetworks(cli *client.Client, ctx context.Context, project *composeTypes.Project) error {
	for networkName, networkConfig := range project.Networks {
		// Check if network already exists
		networks, err := cli.NetworkList(ctx, network.ListOptions{
			Filters: filters.NewArgs(filters.Arg("name", networkName)),
		})
		if err != nil {
			return fmt.Errorf("failed to list networks: %w", err)
		}

		if networkName == "default" {
			log.Printf("Skipping predefined network %q", networkName)
			continue
		}

		// Skip if network already exists
		if len(networks) > 0 {
			log.Printf("Network %s already exists, skipping creation", networkName)
			continue
		}

		// Create network options
		createOptions := network.CreateOptions{
			Driver: "bridge", // default driver
			Labels: map[string]string{
				"com.docker.compose.project": project.Name,
				"com.docker.compose.network": networkName,
			},
		}

		// Apply custom driver if specified
		if networkConfig.Driver != "" {
			createOptions.Driver = networkConfig.Driver
		}

		// Apply custom options if specified
		if networkConfig.DriverOpts != nil {
			createOptions.Options = networkConfig.DriverOpts
		}

		// Create the network
		resp, err := cli.NetworkCreate(ctx, networkName, createOptions)
		if err != nil {
			return fmt.Errorf("failed to create network %s: %w", networkName, err)
		}

		log.Printf("Created network: %s (ID: %s)", networkName, resp.ID)
	}

	return nil
}

// getServiceEnvironment extracts environment variables from service config
func getServiceEnvironment(svc composeTypes.ServiceConfig) []string {
	env := make([]string, 0, len(svc.Environment))
	for key, value := range svc.Environment {
		if value != nil {
			env = append(env, fmt.Sprintf("%s=%s", key, *value))
		}
	}
	return env
}
