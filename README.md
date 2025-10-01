# dperf - Drive Performance Benchmarking Tool

[![Go Report Card](https://goreportcard.com/badge/github.com/minio/dperf)](https://goreportcard.com/report/github.com/minio/dperf)
[![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)

A high-performance drive benchmarking utility designed to identify storage bottlenecks and performance outliers across multiple drives in distributed storage systems.

## Overview

**dperf** is an enterprise-grade storage performance measurement tool that helps system administrators and DevOps teams quickly identify slow or failing drives in production environments. By performing parallel I/O operations across multiple drives and presenting results in a clear, sorted format, dperf enables rapid troubleshooting and capacity planning for storage infrastructure.

### Key Features

- **Parallel Performance Testing**: Simultaneously benchmark multiple drives to identify performance outliers
- **Direct I/O Operations**: Uses O_DIRECT for accurate drive performance measurement, bypassing OS caches
- **Sorted Results**: Automatically ranks drives by throughput, showing fastest drives first
- **Flexible Workloads**: Configurable block sizes, file sizes, and concurrency levels
- **Production-Ready**: Minimal resource footprint with automatic cleanup
- **Enterprise Support**: Multi-architecture Linux support (amd64, arm64, ppc64le, s390x)
- **Kubernetes Native**: Easily deploy as Jobs or DaemonSets for cluster-wide storage validation

### Why Use dperf?

**For Enterprise Operations:**
- **Proactive Monitoring**: Identify failing drives before they impact production workloads
- **Hardware Validation**: Verify new storage hardware meets performance SLAs
- **Capacity Planning**: Establish performance baselines for storage infrastructure
- **Compliance**: Document storage performance for audit and compliance requirements
- **Cost Optimization**: Identify underperforming drives that should be replaced

**For Developers:**
- **CI/CD Integration**: Automated storage performance validation in deployment pipelines
- **Troubleshooting**: Quick diagnosis of I/O performance issues
- **Benchmark Comparisons**: Compare different storage configurations and technologies
- **Minimal Dependencies**: Single binary with no external requirements

## Quick Start

```bash
# Download the latest binary for Linux amd64
wget https://github.com/minio/dperf/releases/latest/download/dperf-linux-amd64 -O dperf
chmod +x dperf

# Benchmark a single drive
./dperf /mnt/drive1

# Benchmark multiple drives in parallel
./dperf /mnt/drive{1..6}
```

## Installation

### Pre-built Binaries

Download the appropriate binary for your architecture:

| OS    | Architecture | Binary                                                                                       |
|:------|:------------:|:--------------------------------------------------------------------------------------------:|
| Linux | amd64        | [linux-amd64](https://github.com/minio/dperf/releases/latest/download/dperf-linux-amd64)     |
| Linux | arm64        | [linux-arm64](https://github.com/minio/dperf/releases/latest/download/dperf-linux-arm64)     |
| Linux | ppc64le      | [linux-ppc64le](https://github.com/minio/dperf/releases/latest/download/dperf-linux-ppc64le) |
| Linux | s390x        | [linux-s390x](https://github.com/minio/dperf/releases/latest/download/dperf-linux-s390x)     |

```bash
# Example: Install on Linux amd64
wget https://github.com/minio/dperf/releases/latest/download/dperf-linux-amd64
sudo install -m 755 dperf-linux-amd64 /usr/local/bin/dperf
```

### Build from Source
Requires Go 1.24 or later. [Install Go](https://golang.org/doc/install) if not already available.

```bash
# Install directly from source
go install github.com/minio/dperf@latest

# Or clone and build
git clone https://github.com/minio/dperf.git
cd dperf
make build
sudo make install
```

## Usage

### Basic Examples

```bash
# Benchmark a single drive
dperf /mnt/drive1

# Benchmark multiple drives in parallel (default mode)
dperf /mnt/drive{1..6}

# Run benchmarks sequentially (one drive at a time)
dperf --serial /mnt/drive{1..6}

# Verbose output showing individual drive statistics
dperf -v /mnt/drive{1..6}

# Write-only benchmark (skip read tests)
dperf --write-only /mnt/drive{1..6}

# Custom block size and file size
dperf -b 8MiB -f 5GiB /mnt/drive{1..6}

# High concurrency test
dperf -i 16 /mnt/drive{1..6}
```

### Command-Line Flags

```
Flags:
  -b, --blocksize string  Read/write block size (default "4MiB")
                          Must be >= 4KiB and a multiple of 4KiB

  -f, --filesize string   Amount of data to read/write per drive (default "1GiB")
                          Must be >= 4KiB and a multiple of 4KiB

  -i, --ioperdrive int    Number of concurrent I/O operations per drive (default 4)
                          Higher values increase parallelism

      --serial            Run tests sequentially instead of in parallel
                          Useful for isolating drive-specific issues

      --write-only        Run write tests only, skip read tests
                          Faster benchmarking when only write performance matters

  -v, --verbose           Show per-drive statistics in addition to aggregate totals

  -h, --help              Display help information
      --version           Show version information
```

### Example Output

```bash
$ dperf /mnt/drive{1..4}

┌────────────────┬──────────────┐
│   TotalWRITE   │  TotalREAD   │
├────────────────┼──────────────┤
│ 4.2 GiB/s      │ 4.5 GiB/s    │
└────────────────┴──────────────┘
```

With verbose output (`-v`):

```bash
$ dperf -v /mnt/drive{1..4}

┌──────────────┬──────────────┬──────────────┬────┐
│     PATH     │    WRITE     │     READ     │    │
├──────────────┼──────────────┼──────────────┼────┤
│ /mnt/drive1  │ 1.1 GiB/s    │ 1.2 GiB/s    │ ✓  │
│ /mnt/drive2  │ 1.0 GiB/s    │ 1.1 GiB/s    │ ✓  │
│ /mnt/drive3  │ 1.1 GiB/s    │ 1.2 GiB/s    │ ✓  │
│ /mnt/drive4  │ 1.0 GiB/s    │ 1.0 GiB/s    │ ✓  │
└──────────────┴──────────────┴──────────────┴────┘

┌────────────────┬──────────────┐
│   TotalWRITE   │  TotalREAD   │
├────────────────┼──────────────┤
│ 4.2 GiB/s      │ 4.5 GiB/s    │
└────────────────┴──────────────┘
```

## Enterprise Use Cases

### 1. Automated Storage Health Checks

Deploy dperf as a Kubernetes CronJob to regularly validate storage performance:

```bash
# Run daily storage health checks
kubectl apply -f - <<EOF
apiVersion: batch/v1
kind: CronJob
metadata:
  name: storage-health-check
spec:
  schedule: "0 2 * * *"
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: dperf
            image: quay.io/minio/dperf:latest
            args: ["/data1", "/data2", "/data3", "/data4"]
            volumeMounts:
            - name: storage
              mountPath: /data
          restartPolicy: OnFailure
EOF
```

### 2. New Hardware Validation

Before adding new storage nodes to production, validate performance meets requirements:

```bash
# Define performance SLA
MIN_WRITE_THROUGHPUT="800MiB/s"
MIN_READ_THROUGHPUT="900MiB/s"

# Run benchmark and validate
dperf /mnt/new-drive{1..8} > results.txt
# Parse results and compare against SLA
```

### 3. Troubleshooting Performance Degradation

Quickly identify the slowest drives in a storage cluster:

```bash
# Sorted output shows slowest drives at the bottom
dperf /mnt/drive{1..100} | tee drive-performance.log
```

### 4. Kubernetes Persistent Volume Validation

Test storage performance for Kubernetes PersistentVolumes before application deployment:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: pv-performance-test
spec:
  template:
    spec:
      restartPolicy: Never
      containers:
      - name: dperf
        image: quay.io/minio/dperf:latest
        args: ["-v", "/data"]
        volumeMounts:
        - name: test-volume
          mountPath: /data
      volumes:
      - name: test-volume
        persistentVolumeClaim:
          claimName: my-pvc
```

## System Requirements

- **Operating System**: Linux (uses Linux-specific direct I/O features)
- **Kernel**: Linux kernel with O_DIRECT support
- **File System**: Any file system supporting direct I/O (ext4, xfs, etc.)
- **Permissions**: Write access to target directories
- **Disk Space**: Temporary space equal to `filesize × ioperdrive` per drive

## How It Works

dperf performs accurate drive performance measurements using several key techniques:

1. **Direct I/O (O_DIRECT)**: Bypasses operating system page cache for accurate hardware measurements
2. **Page-Aligned Buffers**: Uses 4KiB-aligned memory buffers required for direct I/O operations
3. **Parallel Testing**: Launches concurrent I/O operations per drive to measure maximum throughput
4. **Fsync/Fdatasync**: Ensures data is written to physical media, not just cached
5. **Sequential Hints**: Uses `fadvise(FADV_SEQUENTIAL)` for optimized read patterns

The benchmark process:
1. Creates temporary test files in each target directory
2. Writes data using multiple concurrent threads (default: 4 per drive)
3. Reads data back using the same concurrency level
4. Calculates throughput (bytes/second) for both operations
5. Cleans up all temporary files automatically

## Troubleshooting

### Permission Denied Errors

```bash
# Ensure write permissions on target directories
sudo dperf /mnt/restricted-drive

# Or change directory permissions
sudo chmod 755 /mnt/restricted-drive
```

### "Invalid blocksize" Errors

Block size must be at least 4KiB and a multiple of 4KiB due to direct I/O alignment requirements:

```bash
# Invalid
dperf -b 1024 /mnt/drive1  # Too small

# Valid
dperf -b 4KiB /mnt/drive1   # Minimum size
dperf -b 1MiB /mnt/drive1   # Common size
dperf -b 8MiB /mnt/drive1   # Larger blocks
```

### Low Performance Results

- **Check Drive Health**: Use `smartctl` to verify drive health
- **Check CPU Throttling**: Ensure CPU is not throttled during tests
- **Check I/O Scheduler**: Verify appropriate I/O scheduler is configured
- **Increase Concurrency**: Try higher `-i` values for better parallelism
- **Check File System**: Some file systems perform better than others

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request. For major changes, please open an issue first to discuss what you would like to change.

## License

dperf is released under the [GNU Affero General Public License v3.0](https://www.gnu.org/licenses/agpl-3.0). See [LICENSE](LICENSE) for details.

## Support

- **Issues**: [GitHub Issues](https://github.com/minio/dperf/issues)
- **Enterprise Support**: Contact [MinIO Sales](https://min.io/contact)
