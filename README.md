# Tiny Status Page

A lightweight web application that provides a real-time overview of Kubernetes services and their health status.

## Features

- Real-time monitoring of Kubernetes services
- Health status indicators
- Pod endpoint information with clickable IP addresses
- Service port details
- Clean, modern web interface
- Support for both in-cluster and local development
- Automatic cluster/local configuration detection

## Installation

### Prerequisites

- Go 1.22 or later [if building from source]
- Access to a Kubernetes cluster
- `kubectl` configured with appropriate permissions

### Obtaining release binaries

Latest build is available through [GitHub releases page](https://github.com/x0ddf/tiny-status-page/releases).
If you don't see your platform binary consider to build yourselves.

### Running in Kubernetes

Apply the deployment manifest:
```bash
kubectl apply -f deploy/kubernetes.yaml
```


### Building Locally

- Clone the repository:
```bash
git clone https://github.com/x0ddf/tiny-status-page.git
cd tiny-status-page
```
- Build the application:
```bash
go build -o tiny-status-page cmd/backend/main.go
```
- Run the application:
```bash
./tiny-status-page
```

### Container Registry

Images are available at:

- [ghcr.io/x0ddf/tiny-status-page](https://github.com/x0ddf/tiny-status-page/pkgs/container/tiny-status-page)


## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.