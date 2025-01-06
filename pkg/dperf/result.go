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
	"errors"

	"github.com/dustin/go-humanize"
	"github.com/fatih/color"
	"github.com/minio/pkg/v3/console"
)

// ErrNotImplemented returned for platforms where dperf will not run.
var ErrNotImplemented = errors.New("not implemented")

// DrivePerfResult drive run result
type DrivePerfResult struct {
	Path            string
	WriteThroughput uint64
	ReadThroughput  uint64
	Error           error
}

// An alias of string to represent the health color code of an object
type col string

const (
	colGrey   col = "Grey"
	colRed    col = "Red"
	colYellow col = "Yellow"
	colGreen  col = "Green"
)

// getPrintCol - map color code to color for printing
func getPrintCol(c col) *color.Color {
	switch c {
	case colGrey:
		return color.New(color.FgWhite, color.Bold)
	case colRed:
		return color.New(color.FgRed, color.Bold)
	case colYellow:
		return color.New(color.FgYellow, color.Bold)
	case colGreen:
		return color.New(color.FgGreen, color.Bold)
	}
	return nil
}

func (d *DrivePerf) render(results []*DrivePerfResult) {
	dspOrder := []col{colGreen} // Header
	for i := 0; i < len(results); i++ {
		dspOrder = append(dspOrder, colGrey)
	}

	var printColors []*color.Color
	for _, c := range dspOrder {
		printColors = append(printColors, getPrintCol(c))
	}

	tbl := console.NewTable(printColors, []bool{false, false, false, false}, 0)

	cellText := make([][]string, len(results)+1)
	cellText[0] = []string{
		"PATH",
		"WRITE",
		"READ",
		"",
	}

	var aggregateRead uint64
	var aggregateWrite uint64
	for idx, result := range results {
		idx++
		read := humanize.IBytes(result.ReadThroughput) + "/s"
		write := humanize.IBytes(result.WriteThroughput) + "/s"
		aggregateRead += result.ReadThroughput
		aggregateWrite += result.WriteThroughput
		if result.Error != nil {
			read = "-"
			write = "-"
		}

		err := func() string {
			if result.Error != nil {
				return result.Error.Error()
			}
			return "✓"
		}()

		cellText[idx] = []string{
			result.Path,
			write,
			read,
			err,
		}
	}
	if d.Verbose {
		tbl.DisplayTable(cellText)
	}

	dspAggOrder := []col{colGreen, colGrey} // Header
	printColors = []*color.Color{}
	for _, c := range dspAggOrder {
		printColors = append(printColors, getPrintCol(c))
	}

	tblAgg := console.NewTable(printColors, []bool{false, false}, 0)
	cellText = make([][]string, 2)
	cellText[0] = []string{
		"TotalWRITE",
		"TotalREAD",
	}
	cellText[1] = []string{
		humanize.IBytes(aggregateWrite) + "/s",
		humanize.IBytes(aggregateRead) + "/s",
	}
	tblAgg.DisplayTable(cellText)
}
