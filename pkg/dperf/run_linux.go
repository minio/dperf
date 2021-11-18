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
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/ncw/directio"
	"golang.org/x/sys/unix"
)

func (d *DrivePerf) runReadTest(ctx context.Context, path string) (float64, error) {
	f, err := directio.OpenFile(path, os.O_RDONLY, 0400)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	// Read Algined block upto a multiple of BlockSize
	data := directio.AlignedBlock(int(d.BlockSize))

	startTime := time.Now()
	for i := uint64(0); i < (d.FileSize / d.BlockSize); i++ {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
			if n, err := f.Read(data); err != nil {
				return 0, err
			} else if uint64(n) != d.BlockSize {
				return 0, fmt.Errorf("Expected to read %d bytes, but read %d bytes",
					d.BlockSize, n)
			}
		}
	}

	// if total file size does not align to block size
	// disable directIO and read unaligned block
	remainder := d.FileSize % d.BlockSize
	if remainder != 0 {
		fd := f.Fd()
		flag, err := unix.FcntlInt(fd, unix.F_GETFL, 0)
		if err != nil {
			return 0, err
		}

		flag &= ^(syscall.O_DIRECT)
		if _, err := unix.FcntlInt(fd, unix.F_SETFL, flag); err != nil {
			return 0, err
		}

		// write unaligned block
		data := make([]byte, remainder)
		if n, err := f.Read(data); err != nil {
			return 0, err
		} else if uint64(n) != remainder {
			return 0, fmt.Errorf("Expected to read %d bytes, but read %d bytes",
				remainder, n)
		}
	}

	timeTakenInSeconds := time.Since(startTime).Seconds()

	return float64(d.FileSize) / timeTakenInSeconds, nil
}

func (d *DrivePerf) runWriteTest(ctx context.Context, path string) (float64, error) {
	f, err := directio.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	// Write Algined block upto a multiple of BlockSize
	data := directio.AlignedBlock(int(d.BlockSize))

	startTime := time.Now()
	for i := uint64(0); i < (d.FileSize / d.BlockSize); i++ {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
			if n, err := f.Write(data); err != nil {
				return 0, err
			} else if uint64(n) != d.BlockSize {
				return 0, fmt.Errorf("Expected to write %d bytes, but wrote %d bytes",
					d.BlockSize, n)
			}
		}
	}

	// if total file size does not align to block size
	// disable directIO and write unaligned block
	remainder := d.FileSize % d.BlockSize
	if remainder != 0 {
		fd := f.Fd()
		flag, err := unix.FcntlInt(fd, unix.F_GETFL, 0)
		if err != nil {
			return 0, err
		}

		flag &= ^(syscall.O_DIRECT)
		if _, err := unix.FcntlInt(fd, unix.F_SETFL, flag); err != nil {
			return 0, err
		}

		// write unaligned block
		data := make([]byte, remainder)
		if n, err := f.Write(data); err != nil {
			return 0, err
		} else if uint64(n) != remainder {
			return 0, fmt.Errorf("Expected to write %d bytes, but wrote %d bytes",
				remainder, n)
		}
	}

	if err := syscall.Fdatasync(int(f.Fd())); err != nil {
		return 0, err
	}
	timeTakenInSeconds := time.Since(startTime).Seconds()

	return float64(d.FileSize) / timeTakenInSeconds, nil
}
