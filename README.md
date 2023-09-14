# dperf - drive performance

dperf is a drive performance measurement tool to identify slow drives in your host. It takes multiple file paths as input, and performs I/O parallelly on those files. The read and write throughput are printed in sorted order, with the fastest drives shown first.

The tool chooses sensible defaults for parameters such as block size, total bytes to read/write etc. to find I/O bottlenecks.

## Install

### Binary

| OS       | ARCH    | Binary                                                                                       |
|:--------:|:-------:|:--------------------------------------------------------------------------------------------:|
| Linux    | amd64   | [linux-amd64](https://github.com/minio/dperf/releases/latest/download/dperf-linux-amd64)         |
| Linux    | arm64   | [linux-arm64](https://github.com/minio/dperf/releases/latest/download/dperf-linux-arm64)         |
| Linux    | ppc64le | [linux-ppc64le](https://github.com/minio/dperf/releases/latest/download/dperf-linux-ppc64le)     |
| Linux    | s390x   | [linux-s390x](https://github.com/minio/dperf/releases/latest/download/dperf-linux-s390x)         |

### Source

```
go install github.com/minio/dperf@latest
```

> You will need a working Go environment. Therefore, please follow [How to install Go](https://golang.org/doc/install).
> Minimum version required is go1.17

## Usage

```
$ dperf --help

MinIO drive performance utility
--------------------------------
  dperf measures throughput of each of the drives mounted at PATH...

Usage:
  dperf [flags] PATH...

Examples:

# run dpref on drive mounted at /mnt/drive1
λ dperf /mnt/drive1

# run dperf on drives 1 to 6. Output will be sorted by throughput. Fastest drive is at the top.
λ dperf /mnt/drive{1..6}

# run dperf on drives one-by-one
λ dperf --serial /mnt/drive{1...6}

Flags:
  -b, --blocksize string   read/write block size (default "4MiB")
  -f, --filesize string    amount of data to read/write per drive (default "1GiB")
  -i, --ioperdrive int     number of concurrent I/O per drive (default 4)
  -h, --help               help for dperf
      --serial             run tests one by one, instead of all at once.
      --version            version for dperf
```
