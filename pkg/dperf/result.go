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
	"os"

	"github.com/dustin/go-humanize"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
)

// DrivePerfResult drive run result
type DrivePerfResult struct {
	Path            string
	WriteThroughput uint64
	ReadThroughput  uint64
	Error           error
}

func render(results []*DrivePerfResult) {
	headers := []interface{}{
		"PATH",
		"READ",
		"WRITE",
		"",
	}

	text.DisableColors()
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row(headers))

	style := table.StyleColoredDark
	style.Color.IndexColumn = text.Colors{text.FgHiBlue, text.BgHiBlack}
	style.Color.Header = text.Colors{text.FgHiBlue, text.BgHiBlack}
	t.SetStyle(style)

	for _, result := range results {
		read := humanize.IBytes(result.ReadThroughput) + "/s"
		write := humanize.IBytes(result.WriteThroughput) + "/s"
		if result.Error != nil {
			read = "-"
			write = "-"
		}
		err := func() string {
			if result.Error != nil {
				return result.Error.Error()
			}
			return ""
		}()

		output := []interface{}{
			result.Path,
			read,
			write,
			err,
		}

		t.AppendRow(output)
	}
	t.Render()
}
