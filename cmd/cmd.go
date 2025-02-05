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
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/bygui86/multi-profile/v2"
	"github.com/dustin/go-humanize"
	"github.com/felixge/fgprof"
	"github.com/minio/dperf/pkg/dperf"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Version version string for dperf
var Version = "dev"

// O_DIRECT align size.
const (
	alignSize = 4096
	tmpFile   = "..tmpFile"
)

// flags
var (
	serial     = false
	writeOnly  = false
	verbose    = false
	blockSize  = "4MiB"
	fileSize   = "1GiB"
	cpuNode    = 0
	ioPerDrive = 4
	profileDir = "./"

	pCPU, pCPUio, pBlock, pMem, pMutex, pThread, pTrace bool
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
			WriteOnly:  writeOnly,
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
				return errors.New("path '" + path + "' is not a directory")
			}

			if !isDirWritable(path) {
				return errors.New("directory at path '" + path + "' is not writable")
			}

			paths = append(paths, filepath.Clean(arg))
		}
		defer startTraces()()
		return perf.RunAndRender(c.Context(), paths...)
	},
}

func startTraces() func() {
	var profiles []*profile.Profile
	cfg := &profile.Config{
		Path:           profileDir,
		UseTempPath:    false,
		Quiet:          !verbose,
		MemProfileRate: 4096,
		MemProfileType: "heap",
		CloserHook:     nil,
		Logger:         nil,
	}
	type starter interface {
		Start() *profile.Profile
	}
	startIf := func(c bool, s starter) {
		if c {
			profiles = append(profiles, s.Start())
		}
	}
	startIf(pCPU, profile.CPUProfile(cfg))
	startIf(pMem, profile.MemProfile(cfg))
	startIf(pBlock, profile.BlockProfile(cfg))
	startIf(pMutex, profile.MutexProfile(cfg))
	startIf(pTrace, profile.TraceProfile(cfg))
	startIf(pThread, profile.ThreadCreationProfile(cfg))
	var cpuIOBuf bytes.Buffer
	var stopCPUIO func() error
	if pCPUio {
		stopCPUIO = fgprof.Start(&cpuIOBuf, fgprof.FormatPprof)
		if verbose {
			fmt.Println("[info] CPU/IO profiling enabled")
		}
	}
	started := time.Now()
	return func() {
		for _, p := range profiles {
			p.Stop()
		}
		// Light hack around https://github.com/felixge/fgprof/pull/34
		if stopCPUIO != nil && time.Since(started) > 100*time.Millisecond {
			if verbose {
				fmt.Println("[info]  Stop and flush CPU/IO profiling to file", filepath.Join(profileDir, "cpuio.pprof"))
			}
			err := stopCPUIO()
			if err != nil {
				fmt.Printf("Failed to stop CPU IO: %v\n", err)
				return
			}
			err = os.WriteFile(filepath.Join(profileDir, "cpuio.pprof"), cpuIOBuf.Bytes(), 0o666)
			if err != nil {
				fmt.Printf("Failed to write CPU IO profile: %v\n", err)
				return
			}
		}
	}
}

func init() {
	viper.AutomaticEnv()

	// parse the go default flagset to get flags for glog and other packages in future
	dperfCmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)

	flag.Set("logtostderr", "true")
	flag.Set("alsologtostderr", "true")

	dperfCmd.PersistentFlags().BoolVarP(&serial,
		"serial", "", serial, "run tests one by one, instead of all at once")
	dperfCmd.PersistentFlags().BoolVarP(&writeOnly,
		"write-only", "", writeOnly, "run write only tests")
	dperfCmd.PersistentFlags().BoolVarP(&verbose,
		"verbose", "v", verbose, "print READ/WRITE for each paths independently, default only prints aggregated")
	dperfCmd.PersistentFlags().StringVarP(&blockSize,
		"blocksize", "b", blockSize, "read/write block size")
	dperfCmd.PersistentFlags().StringVarP(&fileSize,
		"filesize", "f", fileSize, "amount of data to read/write per drive")
	dperfCmd.PersistentFlags().IntVarP(&ioPerDrive,
		"ioperdrive", "i", ioPerDrive, "number of concurrent I/O per drive, default is 4")

	// Go profiles
	dperfCmd.PersistentFlags().StringVar(&profileDir,
		"prof.dir", profileDir, "save profiles in directory")
	dperfCmd.PersistentFlags().MarkHidden("prof.dir")
	dperfCmd.PersistentFlags().BoolVar(&pCPU,
		"prof.cpu", false, "cpu profile")
	dperfCmd.PersistentFlags().MarkHidden("prof.cpu")
	dperfCmd.PersistentFlags().BoolVar(&pMem,
		"prof.mem", false, "mem profile")
	dperfCmd.PersistentFlags().MarkHidden("prof.mem")
	dperfCmd.PersistentFlags().BoolVar(&pBlock,
		"prof.block", false, "blocking profile")
	dperfCmd.PersistentFlags().MarkHidden("prof.block")
	dperfCmd.PersistentFlags().BoolVar(&pMutex,
		"prof.mutex", false, "mutex profile")
	dperfCmd.PersistentFlags().MarkHidden("prof.mutex")
	dperfCmd.PersistentFlags().BoolVar(&pTrace,
		"prof.trace", false, "trace profile")
	dperfCmd.PersistentFlags().MarkHidden("prof.trace")
	dperfCmd.PersistentFlags().BoolVar(&pThread,
		"prof.thread", false, "thread profile")
	dperfCmd.PersistentFlags().MarkHidden("prof.thread")
	dperfCmd.PersistentFlags().BoolVar(&pCPUio,
		"prof.cpuio", false, "cpuio profile")
	dperfCmd.PersistentFlags().MarkHidden("prof.cpuio")

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

// Check if the directory is writable or not
func isDirWritable(dir string) bool {
	file, err := os.CreateTemp(dir, tmpFile)
	if err != nil {
		return false
	}
	file.Close()
	// clean up
	os.Remove(file.Name())
	return true
}
