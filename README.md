# kunado

A simple TUI port monitor for Linux.

## Features

- Real-time view of listening TCP and UDP ports over IPv4 and IPv6
- Process details, including PID, working directory, and Nerd Font icons
- Automatic detection of Docker containers and Docker Compose projects and services

## Run

```sh
go run ./cmd/kunado
```

## Install

```sh
go install ./cmd/kunado # install at $(go env GOPATH)/bin/kunado

kunado
```

Some process details may require root privileges:

```sh
sudo kunado
```
