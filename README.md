dperf - drive performance
--------------------------

dperf is a drive performance measurement tool to identify slow drives in your host. It takes multiple file paths as input, and performs I/O parallely on those files. The read and write throughput are printed in sorted order, with the fastest drives shown first.

The tool chooses sensible defaults for parameters such as block size, total bytes to read/write etc. to find I/O bottlenecks.

Getting Started
----------------

Run this command to install dperf

```
wget ${DOWNLOAD_URL}
```

If you have a golang dev environment

```
go get github.com/minio/dperf
```

Usage
------

```
$ dperf --help

MinIO drive performance utility
-------------------------------- 
  dperf measures throughput of each of the drives mounted at PATH...

Usage:
  dperf [flags] PATH...

Examples:

# run dpref on drive mounted at /mnt/drive1
$ dperf /mnt/drive1

# run dperf on drives 1 to 6. Output will be sorted by throughput. Fastest drive is at the top. 
$ dperf /mnt/drive{1..6}

# run dperf on drives one-by-one 
$ dperf --serial /mnt/drive{1...6}  


Flags:
  -b, --blocksize string   read/write block size (default "4MiB")
  -f, --filesize string    amount of data to read/write per drive (default "1GiB")
  -h, --help               help for dperf
      --serial             run tests one by one, instead of all at once.
      --version            version for dperf
```
