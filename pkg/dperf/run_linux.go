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
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ncw/directio"
	"golang.org/x/sys/unix"
)

// odirectReader - to support O_DIRECT reads for erasure backends.
type odirectReader struct {
	File     *os.File
	Bufp     *[]byte
	buf      []byte
	err      error
	seenRead bool

	ctx context.Context
}

// Read - Implements Reader interface.
func (o *odirectReader) Read(buf []byte) (n int, err error) {
	if o.ctx.Err() != nil {
		return 0, o.ctx.Err()
	}
	if o.err != nil && (len(o.buf) == 0 || !o.seenRead) {
		return 0, o.err
	}
	if !o.seenRead {
		o.buf = *o.Bufp
		n, err = o.File.Read(o.buf)
		if err != nil && err != io.EOF {
			if errors.Is(err, syscall.EINVAL) {
				if err = disableDirectIO(o.File); err != nil {
					o.err = err
					return n, err
				}
				n, err = o.File.Read(o.buf)
			}
			if err != nil && err != io.EOF {
				o.err = err
				return n, err
			}
		}
		if n == 0 {
			// err is likely io.EOF
			o.err = err
			return n, err
		}
		o.err = err
		o.buf = o.buf[:n]
		o.seenRead = true
	}
	if len(buf) >= len(o.buf) {
		n = copy(buf, o.buf)
		o.seenRead = false
		return n, o.err
	}
	n = copy(buf, o.buf)
	o.buf = o.buf[n:]
	// There is more left in buffer, do not return any EOF yet.
	return n, nil
}

// Close - Release the buffer and close the file.
func (o *odirectReader) Close() error {
	o.err = errors.New("internal error: odirectReader Read after Close")
	fadviseDontNeed(o.File)
	return o.File.Close()
}

func (d *DrivePerf) runReadTest(ctx context.Context, path string) (uint64, error) {
	startTime := time.Now()
	f, err := directio.OpenFile(path, os.O_RDONLY, 0400)
	if err != nil {
		return 0, err
	}

	// Read Aligned block upto a multiple of BlockSize
	data := directio.AlignedBlock(int(d.BlockSize))
	of := &odirectReader{
		File: f,
		Bufp: &data,
		ctx:  ctx,
	}
	n, err := io.Copy(ioutil.Discard, of)
	of.Close()
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

// disableDirectIO - disables directio mode.
func disableDirectIO(f *os.File) error {
	fd := f.Fd()
	flag, err := unix.FcntlInt(fd, unix.F_GETFL, 0)
	if err != nil {
		return err
	}
	flag &= ^(syscall.O_DIRECT)
	_, err = unix.FcntlInt(fd, unix.F_SETFL, flag)
	return err
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
func fdatasync(f *os.File) error {
	return syscall.Fdatasync(int(f.Fd()))
}

// fadviseDontNeed invalidates page-cache
func fadviseDontNeed(f *os.File) error {
	return unix.Fadvise(int(f.Fd()), 0, 0, unix.FADV_DONTNEED)
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

type odirectWriter struct {
	File *os.File
}

func (o *odirectWriter) Close() error {
	fdatasync(o.File)
	return o.File.Close()
}

func (o *odirectWriter) Write(buf []byte) (n int, err error) {
	return o.File.Write(buf)
}

func (d *DrivePerf) runWriteTest(ctx context.Context, path string) (uint64, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return 0, err
	}

	startTime := time.Now()
	f, err := directio.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return 0, err
	}

	// Write Aligned block upto a multiple of BlockSize
	data := alignedBlock(int(d.BlockSize))

	// Use odirectWriter instead of os.File so io.CopyBuffer() will only be aware
	// of a io.Writer interface and will be enforced to use the copy buffer.
	of := &odirectWriter{
		File: f,
	}

	n, err := io.CopyBuffer(of, io.LimitReader(&nullReader{ctx: ctx}, int64(d.FileSize)), data)
	of.Close()
	if err != nil {
		return 0, err
	}

	if n != int64(d.FileSize) {
		return 0, fmt.Errorf("Expected to write %d, wrote %d bytes", d.FileSize, n)
	}

	dt := float64(time.Since(startTime))
	throughputInSeconds := (float64(d.FileSize) / dt) * float64(time.Second)
	return uint64(throughputInSeconds), nil
}
