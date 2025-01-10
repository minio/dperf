// This file is part of MinIO dperf
// Copyright (c) 2021 MinIO, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package dperf

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/minio/pkg/v3/rng"
	"github.com/ncw/directio"
	"golang.org/x/sys/unix"
)

type nullWriter struct{}

func (n nullWriter) Write(b []byte) (int, error) {
	return len(b), nil
}

func (d *DrivePerf) runReadTest(ctx context.Context, path string, data []byte) (uint64, error) {
	startTime := time.Now()
	r, err := os.OpenFile(path, syscall.O_DIRECT|os.O_RDONLY, 0o400)
	if err != nil {
		return 0, err
	}
	unix.Fadvise(int(r.Fd()), 0, int64(d.FileSize), unix.FADV_SEQUENTIAL)

	n, err := copyAligned(&nullWriter{}, r, data, int64(d.FileSize), r.Fd())
	r.Close()
	if err != nil {
		return 0, err
	}
	if n != int64(d.FileSize) {
		return 0, fmt.Errorf("Expected read %d, read %d", d.FileSize, n)
	}

	dt := float64(time.Since(startTime))
	throughputInSeconds := (float64(d.FileSize) / dt) * float64(time.Second)
	return uint64(throughputInSeconds), nil
}

// alignedBlock - pass through to directio implementation.
func alignedBlock(blockSize int) []byte {
	return directio.AlignedBlock(blockSize)
}

// fdatasync - fdatasync() is similar to fsync(), but does not flush modified metadata
// unless that metadata is needed in order to allow a subsequent data retrieval
// to  be  correctly  handled.   For example, changes to st_atime or st_mtime
// (respectively, time of last access and time of last modification; see inode(7))
// do not require flushing because they are not necessary for a subsequent data
// read to be handled correctly. On the other hand, a change to the file size
// (st_size, as made by say ftruncate(2)), would require a metadata flush.
//
// The aim of fdatasync() is to reduce disk activity for applications that
// do not require all metadata to be synchronized with the disk.
func fdatasync(fd int) error {
	return syscall.Fdatasync(fd)
}

func fadviseSequential(f *os.File, length int64) error {
	return unix.Fadvise(int(f.Fd()), 0, length, unix.FADV_SEQUENTIAL)
}

type nullReader struct {
	ctx context.Context
}

func (n nullReader) Read(b []byte) (int, error) {
	if n.ctx.Err() != nil {
		return 0, n.ctx.Err()
	}
	return len(b), nil
}

func newRandomReader(ctx context.Context) io.Reader {
	r, err := rng.NewReader()
	if err != nil {
		panic(err)
	}
	return r
}

// disableDirectIO - disables directio mode.
func disableDirectIO(fd uintptr) error {
	flag, err := unix.FcntlInt(fd, unix.F_GETFL, 0)
	if err != nil {
		return err
	}
	flag &= ^(syscall.O_DIRECT)
	_, err = unix.FcntlInt(fd, unix.F_SETFL, flag)
	return err
}

// DirectioAlignSize - DirectIO alignment needs to be 4K. Defined here as
// directio.AlignSize is defined as 0 in MacOS causing divide by 0 error.
const DirectioAlignSize = 4096

// copyAligned - copies from reader to writer using the aligned input
// buffer, it is expected that input buffer is page aligned to
// 4K page boundaries. Without passing aligned buffer may cause
// this function to return error.
//
// This code is similar in spirit to io.Copy but it is only to be
// used with DIRECT I/O based file descriptor and it is expected that
// input writer *os.File not a generic io.Writer. Make sure to have
// the file opened for writes with syscall.O_DIRECT flag.
func copyAligned(w io.Writer, r io.Reader, alignedBuf []byte, totalSize int64, fd uintptr) (int64, error) {
	if totalSize == 0 {
		return 0, nil
	}

	var written int64
	for {
		buf := alignedBuf
		if totalSize > 0 {
			remaining := totalSize - written
			if remaining < int64(len(buf)) {
				buf = buf[:remaining]
			}
		}

		if len(buf)%DirectioAlignSize != 0 {
			// Disable O_DIRECT on fd's on unaligned buffer
			// perform an amortized Fdatasync(fd) on the fd at
			// the end, this is performed by the caller before
			// closing 'w'.
			if err := disableDirectIO(fd); err != nil {
				return written, err
			}
		}

		nr, err := io.ReadFull(r, buf)
		eof := errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)
		if err != nil && !eof {
			return written, err
		}

		buf = buf[:nr]
		var (
			n  int
			un int
			nw int64
		)

		remain := len(buf) % DirectioAlignSize
		if remain == 0 {
			// buf is aligned for directio write()
			n, err = w.Write(buf)
			nw = int64(n)
		} else {
			if remain < len(buf) {
				n, err = w.Write(buf[:len(buf)-remain])
				if err != nil {
					return written, err
				}
				nw = int64(n)
			}

			// Disable O_DIRECT on fd's on unaligned buffer
			// perform an amortized Fdatasync(fd) on the fd at
			// the end, this is performed by the caller before
			// closing 'w'.
			if err = disableDirectIO(fd); err != nil {
				return written, err
			}

			// buf is not aligned, hence use writeUnaligned()
			// for the remainder
			un, err = w.Write(buf[len(buf)-remain:])
			nw += int64(un)
		}

		if nw > 0 {
			written += nw
		}

		if err != nil {
			return written, err
		}

		if nw != int64(len(buf)) {
			return written, io.ErrShortWrite
		}

		if totalSize > 0 && written == totalSize {
			// we have written the entire stream, return right here.
			return written, nil
		}

		if eof {
			// We reached EOF prematurely but we did not write everything
			// that we promised that we would write.
			if totalSize > 0 && written != totalSize {
				return written, io.ErrUnexpectedEOF
			}
			return written, nil
		}
	}
}

func (d *DrivePerf) runWriteTest(ctx context.Context, path string, data []byte) (uint64, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return 0, err
	}

	startTime := time.Now()
	w, err := os.OpenFile(path, syscall.O_DIRECT|os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return 0, err
	}

	n, err := copyAligned(w, newRandomReader(ctx), data, int64(d.FileSize), w.Fd())
	if err != nil {
		w.Close()
		return 0, err
	}

	if n != int64(d.FileSize) {
		w.Close()
		return 0, fmt.Errorf("Expected to write %d, wrote %d bytes", d.FileSize, n)
	}

	if err := fdatasync(int(w.Fd())); err != nil {
		return 0, err
	}

	if err := w.Close(); err != nil {
		return 0, err
	}

	dt := float64(time.Since(startTime))
	throughputInSeconds := (float64(d.FileSize) / dt) * float64(time.Second)
	return uint64(throughputInSeconds), nil
}
