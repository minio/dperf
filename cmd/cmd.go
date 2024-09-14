// This file is part of MinIO dperf
// Copyright (c) 2021-2024 MinIO, Inc.
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

package cmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/minio/dperf/pkg/dperf"
)

// Version version string for dperf
var Version = "dev"

// O_DIRECT align size.
const alignSize = 4096

// flags
var (
	serial     = false
	verbose    = false
	blockSize  = "4MiB"
	fileSize   = "1GiB"
	cpuNode    = 0
	ioPerDrive = 4
)

var dperfCmd = &cobra.Command{
	Use:   "dperf [flags] PATH...",
	Short: "MinIO drive performance utility",
	Long: `
MinIO drive performance utility
--------------------------------
  dperf measures throughput of each of the drives mounted at PATH...
`,
	SilenceUsage:  true,
	SilenceErrors: true,
	Args:          cobra.MinimumNArgs(1),
	Version:       Version,
	Example: `
# run dpref on drive mounted at /mnt/drive1
$ dperf /mnt/drive1

# run dperf on drives 1 to 6. Output will be sorted by throughput. Fastest drive is at the top.
$ dperf /mnt/drive{1..6}

# run dperf on drives one-by-one
$ dperf --serial /mnt/drive{1..6}
`,
	RunE: func(c *cobra.Command, args []string) error {
		bs, err := humanize.ParseBytes(blockSize)
		if err != nil {
			return fmt.Errorf("Invalid blocksize format: %v", err)
		}

		if bs < alignSize {
			return fmt.Errorf("Invalid blocksize must greater than 4k: %d", bs)
		}

		if bs%alignSize != 0 {
			return fmt.Errorf("Invalid blocksize must be multiples of 4k: %d", bs)
		}

		fs, err := humanize.ParseBytes(fileSize)
		if err != nil {
			return fmt.Errorf("Invalid filesize format: %v", err)
		}

		if fs < alignSize {
			return fmt.Errorf("Invalid filesize must greater than 4k: %d", fs)
		}

		if fs%alignSize != 0 {
			return fmt.Errorf("Invalid filesize must multiples of 4k: %d", fs)
		}

		if ioPerDrive <= 0 {
			return fmt.Errorf("Invalid ioperdrive must greater than 0: %d", ioPerDrive)
		}

		perf := &dperf.DrivePerf{
			Serial:     serial,
			BlockSize:  bs,
			FileSize:   fs,
			Verbose:    verbose,
			IOPerDrive: ioPerDrive,
		}
		paths := make([]string, 0, len(args))
		for _, arg := range args {
			if filepath.Clean(arg) == "" {
				return errors.New("empty paths are not allowed as input")
			}
			if filepath.Clean(arg) == "/" {
				return errors.New("not allowed to write at the root of the system, please choose a valid path")
			}
			path := filepath.Clean(arg)

			stat, err := os.Stat(path)
			if err != nil {
				if os.IsNotExist(err) {
					return errors.New("directory at path '" + path + "' does not exist")
				}
				return err
			}

			if !stat.Mode().IsDir() {
				return errors.New("path '" + path + "' is not a directory ")
			}
			paths = append(paths, filepath.Clean(arg))
		}
		return perf.RunAndRender(c.Context(), paths...)
	},
}

func init() {
	viper.AutomaticEnv()

	// parse the go default flagset to get flags for glog and other packages in future
	dperfCmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)

	flag.Set("logtostderr", "true")
	flag.Set("alsologtostderr", "true")

	dperfCmd.PersistentFlags().BoolVarP(&serial,
		"serial", "", serial, "run tests one by one, instead of all at once")
	dperfCmd.PersistentFlags().BoolVarP(&verbose,
		"verbose", "", verbose, "print READ/WRITE for each paths independently, default only prints aggregated")
	dperfCmd.PersistentFlags().StringVarP(&blockSize,
		"blocksize", "b", blockSize, "read/write block size")
	dperfCmd.PersistentFlags().StringVarP(&fileSize,
		"filesize", "f", fileSize, "amount of data to read/write per drive")
	dperfCmd.PersistentFlags().IntVarP(&ioPerDrive,
		"ioperdrive", "i", ioPerDrive, "number of concurrent I/O per drive, default is 4")

	dperfCmd.PersistentFlags().MarkHidden("alsologtostderr")
	dperfCmd.PersistentFlags().MarkHidden("add_dir_header")
	dperfCmd.PersistentFlags().MarkHidden("log_backtrace_at")
	dperfCmd.PersistentFlags().MarkHidden("log_dir")
	dperfCmd.PersistentFlags().MarkHidden("log_file")
	dperfCmd.PersistentFlags().MarkHidden("log_file_max_size")
	dperfCmd.PersistentFlags().MarkHidden("logtostderr")
	dperfCmd.PersistentFlags().MarkHidden("master")
	dperfCmd.PersistentFlags().MarkHidden("one_output")
	dperfCmd.PersistentFlags().MarkHidden("skip_headers")
	dperfCmd.PersistentFlags().MarkHidden("skip_log_headers")
	dperfCmd.PersistentFlags().MarkHidden("stderrthreshold")
	dperfCmd.PersistentFlags().MarkHidden("vmodule")
	dperfCmd.PersistentFlags().MarkHidden("v")

	// suppress the incorrect prefix in glog output
	flag.CommandLine.Parse([]string{})
	viper.BindPFlags(dperfCmd.PersistentFlags())
}

// Execute executes plugin command.
func Execute(ctx context.Context) error {
	return dperfCmd.ExecuteContext(ctx)
}
