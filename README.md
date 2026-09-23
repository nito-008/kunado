# kunado

<img width="2551" height="684" alt="image" src="https://github.com/user-attachments/assets/bc077da0-ca58-4289-95b8-cc6704ee939e" />

A simple TUI port monitor for Linux.

## Overview

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
