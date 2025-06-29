package config

import (
    "sync"
    "github.com/xaydras-2/loadBalancer/App/structers"
    "time"
)

var (
    BackendsMu sync.Mutex
    Backends   []*structers.Backend
    Containers []string // real Docker container IDs
    ReqCount   int64    // atomic request counter
)

const (
    ImageName          = "api_load_test:latest"
    ContainerPort      = "8080"
    StartPort          = 9000
    InitialReplicas    = 2
    MaxReplicas        = 5
    MinReplicas        = 1
    ScaleUpThreshold   = 20 // requests per interval
    ScaleDownThreshold = 5  // requests per interval
    ScaleInterval      = 15 * time.Second
    ScaleIntervalAM    = 30 * time.Second
)