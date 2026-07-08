# EdgeNet Node Manager

EdgeNet Node Manager is a modular Go agent that runs as a systemd service on nodes contributed to the EdgeNet project. It handles node registration, network setup (WireGuard), and cluster provisioning.

Documentation: [wiki](https://github.com/EdgeNet-project/nodemanager/wiki)

## Features

- **Modular Provisioning**: Support for multiple backends (Kubernetes, etc.) via a plugin interface.
- **Stable Identity**: Automatic generation and persistence of WireGuard keypairs and node identity.
- **Idempotent Operations**: Safe to restart; the agent checks existing state before applying changes.
- **Structured Logging**: Uses `zap` for high-performance, structured logging.
- **Packaging Support**: Easily create `.deb` and `.rpm` packages.

## Getting Started

### Prerequisites

- Go 1.24 or later
- Linux (for full functionality including WireGuard and provisioning)

### Installation

To build and install the node manager on your system:

```bash
make build
sudo make install
```

### Building Packages

To create RPM and Debian packages:

```bash
make packages
```

## Configuration

The agent's configuration is managed through flags, environment variables, and a configuration file (defaulting to `/etc/edgenet/agent.conf`).

Precedence: Flags > Environment Variables > Config File

## Development

### Running Tests

```bash
make test
```

### Linting

```bash
make lint
```

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Contributors

- Ciro Scognamiglio · [github](https://github.com/CiroScognamiglio)


- dioptra · [web](https://dioptra.io) | [github](https://github.com/dioptra-io) 
- cslash · [web](https://cslash.com) | [github](https://github.com/cslash)

## Copyright

Copyright © 2026 Sorbonne Université.

See the LICENSE file for licensing details.
