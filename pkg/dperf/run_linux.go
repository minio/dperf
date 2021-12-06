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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
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
}

// Read - Implements Reader interface.
func (o *odirectReader) Read(buf []byte) (n int, err error) {
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

func (d *DrivePerf) runReadTest(ctx context.Context, path string) (float64, error) {
	startTime := time.Now()
	f, err := directio.OpenFile(path, os.O_RDONLY, 0400)
	if err != nil {
		return 0, err
	}

	// Read Aligned block upto a multiple of BlockSize
	data := directio.AlignedBlock(int(d.BlockSize))
	of := &odirectReader{File: f, Bufp: &data}
	defer of.Close()
	n, err := io.Copy(ioutil.Discard, of)
	if err != nil {
		return 0, err
	}
	if n != int64(d.FileSize) {
		return 0, fmt.Errorf("Expected read %d, read %d", d.FileSize, n)
	}

	timeTakenInSeconds := time.Since(startTime).Seconds()
	return float64(d.FileSize) / timeTakenInSeconds, nil
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

// directioAlignSize - DirectIO alignment needs to be 4K. Defined here as
// directio.AlignSize is defined as 0 in MacOS causing divide by 0 error.
const directioAlignSize = 4096

// copyAligned - copies from reader to writer using the aligned input
// buffer, it is expected that input buffer is page aligned to
// 4K page boundaries. Without passing aligned buffer may cause
// this function to return error.
//
// This code is similar in spirit to io.Copy but it is only to be
// used with DIRECT I/O based file descriptor and it is expected that
// input writer *os.File not a generic io.Writer. Make sure to have
// the file opened for writes with syscall.O_DIRECT flag.
func copyAligned(w *os.File, r io.Reader, alignedBuf []byte, totalSize int64) (int64, error) {
	// Writes remaining bytes in the buffer.
	writeUnaligned := func(w *os.File, buf []byte) (remainingWritten int64, err error) {
		// disable O_DIRECT on fd's on unaligned buffer
		// perform an amortized Fdatasync(fd) on the fd at
		// the end, this is performed by the caller before
		// closing 'w'.
		if err = disableDirectIO(w); err != nil {
			return remainingWritten, err
		}
		// Since w is *os.File io.Copy shall use ReadFrom() call.
		return io.Copy(w, bytes.NewReader(buf))
	}

	var written int64
	for {
		buf := alignedBuf
		if totalSize != -1 {
			remaining := totalSize - written
			if remaining < int64(len(buf)) {
				buf = buf[:remaining]
			}
		}
		nr, err := io.ReadFull(r, buf)
		eof := err == io.EOF || err == io.ErrUnexpectedEOF
		if err != nil && !eof {
			return written, err
		}

		buf = buf[:nr]
		var nw int64
		if len(buf)%directioAlignSize == 0 {
			var n int
			// buf is aligned for directio write()
			n, err = w.Write(buf)
			nw = int64(n)
		} else {
			// buf is not aligned, hence use writeUnaligned()
			nw, err = writeUnaligned(w, buf)
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

		if totalSize != -1 {
			if written == totalSize {
				return written, nil
			}
		}
		if eof {
			return written, nil
		}
	}
}

type nullReader struct{}

func (n nullReader) Read(b []byte) (int, error) {
	return len(b), nil
}

func (d *DrivePerf) runWriteTest(ctx context.Context, path string) (float64, error) {
	startTime := time.Now()
	f, err := directio.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return 0, err
	}
	defer func() {
		fdatasync(f)
		f.Close()
	}()

	// Write Aligned block upto a multiple of BlockSize
	data := alignedBlock(int(d.BlockSize))
	n, err := copyAligned(f, io.LimitReader(&nullReader{}, int64(d.FileSize)), data, int64(d.FileSize))
	if err != nil {
		return 0, err
	}
	if n != int64(d.FileSize) {
		return 0, fmt.Errorf("Expected to write %d, wrote %d bytes", d.FileSize, n)
	}
	timeTakenInSeconds := time.Since(startTime).Seconds()

	return float64(d.FileSize) / timeTakenInSeconds, nil
}
