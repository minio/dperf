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
	"os"
	"path/filepath"
	"sort"
)

// DrivePerf options
type DrivePerf struct {
	Serial    bool
	BlockSize uint64
	FileSize  uint64
}

// Run drive performance
func (d *DrivePerf) Run(ctx context.Context, paths ...string) error {
	parallelism := len(paths)
	if d.Serial {
		parallelism = 1
	}

	threads := make(chan struct{}, parallelism)
	defer close(threads)

	resultChan := make(chan *DrivePerfResult)
	defer close(resultChan)

	childCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	for _, path := range paths {
		go func(path string) {
			defer os.RemoveAll(path)

			threads <- struct{}{}

			writeThroughput, err := d.runWriteTest(childCtx, path)
			if err != nil {
				resultChan <- &DrivePerfResult{
					Path:  path,
					Error: err,
				}
			}
			readThroughput, err := d.runReadTest(childCtx, path)
			if err != nil {
				resultChan <- &DrivePerfResult{
					Path:  path,
					Error: err,
				}
			}
			resultChan <- &DrivePerfResult{
				Path:            path,
				ReadThroughput:  uint64(readThroughput),
				WriteThroughput: uint64(writeThroughput),
			}
			<-threads
		}(filepath.Clean(path))
	}

	results := []*DrivePerfResult{}
	for i := 0; i < len(paths); i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case result := <-resultChan:
			results = append(results, result)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].ReadThroughput > results[j].ReadThroughput
	})

	render(results)
	return nil
}
