package config

import (
    "sync"
    "github.com/xaydras-2/loadBalancer/App/structers"
    "time"
)

var (
    BackendsMu sync.Mutex
    Backends   structers.BackendHeap
    ReqCount   int64    // atomic request counter
)

const (
    ImageName          = "api_load_test:latest"
    ContainerPort      = "8080"
    InitialReplicas    = 2
    MaxReplicas        = 5
    MinReplicas        = 1
    ScaleUpThreshold   = 20 // requests per interval
    ScaleDownThreshold = 5  // requests per interval
    ScaleInterval      = 15 * time.Second
    ScaleIntervalAM    = 33 * time.Second
)