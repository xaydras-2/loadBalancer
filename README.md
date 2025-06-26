# Load Balancer

A Go-based load balancer that automatically scales Docker containers based on traffic demands, distributes incoming HTTP requests across multiple backend servers, and offers monitoring and reporting capabilities.

## Features

- **HTTP Request Distribution**: Routes incoming requests using Round Robbin.
- **Auto-Scaling**: Dynamically scales up or down Docker container replicas based on configurable traffic thresholds.
- **Monitoring & Reporting**: Tracks request rates, latency, and container counts; generates performance reports.
- **Load Testing Integration**: Includes K6 scripts for performance evaluation.

## Requirements

- Go (version 1.16 or later)
- Docker
- K6 (for load testing)

## Installation

1. **Clone the repository**
   ```bash
   git clone https://github.com/xaydras-2/loadBalancer.git
   cd loadBalancer
   ```

2. **Build the application**
   ```bash
   go build -o loadBalancer .
   ```

## Usage

1. **Start the load balancer**
   ```bash
   ./loadBalancer
   ```

2. **Access the service**
   The load balancer listens on port **8080** by default:
   ```
   http://localhost:8080
   ```

## Configuration

Configuration parameters can be adjusted in the source files:

| Parameter              | Location                        | Default             |
|------------------------|---------------------------------|---------------------|
| Scaling Up Threshold   | `main.go`                       | 20 requests/interval|
| Scaling Down Threshold | `main.go`                       | 5 requests/interval |
| Scale Interval         | `main.go`                       | 15s                 |
| Listening Port         | `main.go`                       | 8080                |

## Auto-Scaling

The auto-scaling logic is implemented in `App/functions/auto_scaling.go`. It:

1. Create a new Docker container is spawned.
2. Can remove a container by it's id.

## Load Balancer

The LB logic is implemented in `App/functions/loadBalancer.go`. It:

1. handle traffic via a proxy handler
2. Check the health of each backend using Round Robbin(RR) method
3. it picks the healthiest backend
4. if none it returns an error

## The main file
(e.g i know that it isn't ideal to load up the main with function, i know that each function and code block should be in a different file for readability)
The LB logic is implemented in `App/main.go`. It:

1. it demands to the auto_scaling to create a given initial set of containers
2. it check if the LB is always handling the traffic, if an error accrued because of no free Backend it signals the auto_scaling to create a new backend
3. if the load has been reduced, it tells the auto_scaling to remove a container by it's id.

## Load Testing

Load testing is performed using K6. The default script is located at `Test/loadtest.js`.

1. **Install K6**
   ```bash
   # macOS
   brew install k6

   # Windows
   choco install k6

   # Linux
   sudo apt-get install k6
   ```

2. **Run the test**
   ```bash
   k6 run --out json=report.json Test/loadtest.js
   ```

3. **View the report**
   After completion, open the HTML report:
   ```bash
   open Test/summary.html
   ```

### Default Load Test Parameters

- Ramp up to **50 VUs** over **10s**
- Hold **50 VUs** for **30s**
- Ramp down to **0 VUs** over **10s**
- Threshold: **95%** of requests under **500ms**

Modify `Test/loadtest.js` to change these settings.

## Quick note

it's still under development, i know that it can be more optimized.

## Contributing

Contributions are welcome! Please:

1. Fork the repo
2. Create a feature branch (`git checkout -b feature/my-feature`)
3. Commit your changes (`git commit -m 'Add feature'`)
4. Push to the branch (`git push origin feature/my-feature`)
5. Open a Pull Request

## License

This project is licensed under the Apache License 2.0. See the [LICENSE](LICENSE) file for details.

## Contact

Maintained by **Belqadi Ayman**. Feel free to open issues or reach out via GitHub.


