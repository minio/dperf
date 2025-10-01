# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**dperf** is a MinIO drive performance measurement tool that identifies slow drives by performing parallel I/O operations on multiple file paths. It measures read/write throughput and displays results sorted by performance (fastest drives first).

## Build & Test Commands

### Build
```bash
make build           # Builds dperf binary with CGO_ENABLED=0
```

### Install
```bash
make install         # Builds and installs to $GOPATH/bin/dperf
go install github.com/minio/dperf@latest  # Install from source
```

### Run
```bash
./dperf /mnt/drive1                    # Single drive
./dperf /mnt/drive{1..6}               # Multiple drives (parallel)
./dperf --serial /mnt/drive{1..6}      # Multiple drives (sequential)
./dperf --write-only /mnt/drive1       # Write-only benchmark
./dperf -v /mnt/drive{1..6}            # Verbose output (per-drive stats)
```

### Key Flags
- `-b, --blocksize`: Read/write block size (default: "4MiB")
- `-f, --filesize`: Amount of data per drive (default: "1GiB")
- `-i, --ioperdrive`: Concurrent I/O per drive (default: 4)
- `--serial`: Run tests sequentially instead of parallel
- `--write-only`: Run write-only tests
- `-v, --verbose`: Show individual path stats (default shows only aggregate)

### Profiling (Hidden Flags)
```bash
./dperf --prof.cpu --prof.dir=./profiles /mnt/drive1   # CPU profiling
./dperf --prof.mem --prof.dir=./profiles /mnt/drive1   # Memory profiling
./dperf --prof.cpuio --prof.dir=./profiles /mnt/drive1 # CPU/IO profiling
```

Other profile types: `--prof.block`, `--prof.mutex`, `--prof.trace`, `--prof.thread`

### Clean
```bash
make clean           # Remove *.test and temporary files
```

## Architecture

### Package Structure

#### `main.go`
Entry point that sets up signal handling (SIGINT, SIGTERM, SIGSEGV) and calls into the `cmd` package.

#### `cmd/cmd.go`
- Defines the Cobra command structure and all CLI flags
- Validates input parameters (blocksize/filesize must be ≥4K and multiples of 4K)
- Validates paths (must be directories, not root, must exist)
- Orchestrates profiling setup via `startTraces()`
- Creates `DrivePerf` struct and calls `RunAndRender()`

#### `pkg/dperf/perf.go`
Core performance testing logic:
- `DrivePerf`: Main configuration struct with Serial, BlockSize, FileSize, IOPerDrive, WriteOnly, Verbose options
- `Run()`: Executes tests either serially or in parallel (goroutines per path)
- `runTests()`: Per-path orchestration - launches IOPerDrive goroutines for write, then read
- `RunAndRender()`: Runs tests and displays sorted results

#### `pkg/dperf/run_linux.go` (Linux only)
Platform-specific I/O implementation using direct I/O (O_DIRECT):
- `runWriteTest()`: Opens file with O_DIRECT|O_RDWR|O_CREATE, writes FileSize bytes using random data, measures throughput
- `runReadTest()`: Opens file with O_DIRECT|O_RDONLY, reads FileSize bytes, measures throughput
- `copyAligned()`: Core I/O function handling aligned/unaligned buffers for direct I/O
- Uses `syscall.Fdatasync()` for write durability
- Uses `unix.Fadvise(FADV_SEQUENTIAL)` for read optimization
- Random data generation via `github.com/minio/pkg/v3/rng`

#### `pkg/dperf/run_other.go` (Non-Linux)
Stub implementation returning `ErrNotImplemented` - dperf only works on Linux.

#### `pkg/dperf/result.go`
Output formatting:
- `DrivePerfResult`: Contains Path, WriteThroughput, ReadThroughput, Error
- `render()`: Displays results in colored tables using `github.com/minio/pkg/v3/console`
- Shows per-drive stats in verbose mode, always shows aggregate TotalWRITE/TotalREAD

### Key Technical Details

**Direct I/O Requirements:**
- Block size must be ≥4096 bytes and a multiple of 4096 (O_DIRECT alignment requirement)
- File size must be ≥4096 bytes and a multiple of 4096
- Buffers allocated via `directio.AlignedBlock()` for page alignment
- When unaligned writes occur, O_DIRECT is disabled and fdatasync is used

**Concurrency Model:**
- By default, runs all paths in parallel (goroutine per path)
- Each path spawns IOPerDrive goroutines (default: 4) for concurrent I/O
- `--serial` flag forces sequential path execution
- Write tests complete before read tests begin (per path)

**Test Files:**
- Created at `{path}/{uuid}/.writable-check.tmp-{0..IOPerDrive-1}`
- Automatically cleaned up after test via `defer os.RemoveAll()`

**Result Sorting:**
- Results sorted by ReadThroughput descending (fastest first)
- Helps identify slowest drives quickly

## Kubernetes Deployment

See `dperf.yaml` for example Job that benchmarks PersistentVolumeClaims (useful for DirectPV testing).

## Requirements

- Linux OS (uses O_DIRECT, unix.Fadvise, syscall.Fdatasync)
- Go 1.17+ for building
- Write permissions on target paths
- Block devices supporting direct I/O
