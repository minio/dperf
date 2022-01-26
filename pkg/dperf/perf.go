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
	"sort"
	"sync"
)

// DrivePerf options
type DrivePerf struct {
	Serial    bool
	Verbose   bool
	BlockSize uint64
	FileSize  uint64
}

func (d *DrivePerf) runTests(ctx context.Context, path string) *DrivePerfResult {
	defer os.RemoveAll(path)

	writeThroughput, err := d.runWriteTest(ctx, path)
	if err != nil {
		return &DrivePerfResult{
			Path:  path,
			Error: err,
		}
	}
	readThroughput, err := d.runReadTest(ctx, path)
	if err != nil {
		return &DrivePerfResult{
			Path:  path,
			Error: err,
		}
	}
	return &DrivePerfResult{
		Path:            path,
		ReadThroughput:  uint64(readThroughput),
		WriteThroughput: uint64(writeThroughput),
	}
}

// Run drive performance
func (d *DrivePerf) Run(ctx context.Context, paths ...string) ([]*DrivePerfResult, error) {
	parallelism := len(paths)
	if d.Serial {
		parallelism = 1
	}

	childCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make([]*DrivePerfResult, len(paths))
	if d.Serial {
		for i, path := range paths {
			results[i] = d.runTests(childCtx, path)
		}
	} else {
		var wg sync.WaitGroup
		wg.Add(parallelism)
		for i, path := range paths {
			go func(idx int, path string) {
				defer wg.Done()
				results[idx] = d.runTests(childCtx, path)
			}(i, path)
		}
		wg.Wait()
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].ReadThroughput > results[j].ReadThroughput
	})

	return results, nil
}

// Run drive performance and render it
func (d *DrivePerf) RunAndRender(ctx context.Context, paths ...string) error {
	results, err := d.Run(ctx, paths...)
	if err != nil {
		return err
	}
	d.render(results)
	return nil
}
