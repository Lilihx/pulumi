// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package integration

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi-fabric/pkg/util/contract"
)

// LumiProgramTestOptions provides options for LumiProgramTest
type LumiProgramTestOptions struct {
	// Dir is the program directory to test.
	Dir string
	// Array of NPM packages which must be `yarn linked` (e.g. {"@lumi/lumi", "@lumi/aws"})
	Dependencies []string
	// Map of config keys and values to set on the Lumi environment (e.g. {"aws:config:region": "us-east-2"})
	Config map[string]string
	// EditDirs is an optional list of edits to apply to the example, as subsequent deployments.
	EditDirs []string

	// Stdout is the writer to use for all stdout messages.
	Stdout io.Writer
	// Stderr is the writer to use for all stderr messages.
	Stderr io.Writer

	// LumiBin is a location of a `lumi` executable to be run.  Taken from the $PATH if missing.
	LumiBin string
	// LumiJSBin is a location of a `lumi` executable to be run.  Taken from the $PATH if missing.
	LumiJSBin string
	// YarnBin is a location of a `yarn` executable to be run.  Taken from the $PATH if missing.
	YarnBin string
}

// With combines a source set of options with a set of overrides.
func (opts LumiProgramTestOptions) With(overrides LumiProgramTestOptions) LumiProgramTestOptions {
	if overrides.Dir != "" {
		opts.Dir = overrides.Dir
	}
	if overrides.Dependencies != nil {
		opts.Dependencies = overrides.Dependencies
	}
	for k, v := range overrides.Config {
		if opts.Config == nil {
			opts.Config = make(map[string]string)
		}
		opts.Config[k] = v
	}
	if overrides.EditDirs != nil {
		opts.EditDirs = overrides.EditDirs
	}
	return opts
}

// LumiProgramTest runs a lifecylce of Lumi commands in a Lumi program working directory.
// Uses the `lumijs`, `lumi`, and `yarn` binaries available on PATH. Executes the following
// workflow:
//   yarn link <each opts.Depencies>
//   lumijs --verbose
//   lumi env init integrationtesting
//   lumi config <each opts.Config>
//   lumi plan
//   lumi deploy
//   lumi plan (expected to be empty)
//   lumi deploy (expected to be empty)
//   lumi destroy --yes
//   lumi env rm --yes integrationtesting
// All commands must return success return codes for the test to succeed.
func LumiProgramTest(t *testing.T, opts LumiProgramTestOptions) {
	t.Parallel()

	// Ensure the required programs are present.
	if opts.LumiBin == "" {
		lumi, err := exec.LookPath("lumi")
		if !assert.NoError(t, err, "Expected to find `lumi` binary on $PATH: %v", err) {
			return
		}
		opts.LumiBin = lumi
	}
	if opts.LumiJSBin == "" {
		lumijs, err := exec.LookPath("lumijs")
		if !assert.NoError(t, err, "Expected to find `lumijs` binary on $PATH: %v", err) {
			return
		}
		opts.LumiJSBin = lumijs
	}
	if opts.YarnBin == "" {
		yarn, err := exec.LookPath("yarn")
		if !assert.NoError(t, err, "Expected to find `yarn` binary on $PATH: %v", err) {
			return
		}
		opts.YarnBin = yarn
	}

	// Set up a prefix so that all output has the test directory name in it.  This is important for debugging
	// because we run tests in parallel, and so all output will be interleaved and difficult to follow otherwise.
	dir := opts.Dir
	prefix := fmt.Sprintf("[ %30.30s ] ", dir[len(dir)-30:])
	stdout := opts.Stdout
	if stdout == nil {
		stdout = newPrefixer(os.Stdout, prefix)
		opts.Stdout = stdout
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = newPrefixer(os.Stderr, prefix)
		opts.Stderr = stderr
	}

	var err error
	_, err = fmt.Fprintf(opts.Stdout, "sample: %v\n", dir)
	contract.IgnoreError(err)
	_, err = fmt.Fprintf(opts.Stdout, "lumi: %v\n", opts.LumiBin)
	contract.IgnoreError(err)
	_, err = fmt.Fprintf(opts.Stdout, "lumijs: %v\n", opts.LumiJSBin)
	contract.IgnoreError(err)
	_, err = fmt.Fprintf(opts.Stdout, "yarn: %v\n", opts.YarnBin)
	contract.IgnoreError(err)

	// Now copy the source project, excluding the .lumi directory.
	dir, err = prepareProject(t, dir, "", opts)
	if !assert.NoError(t, err, "Failed to copy source project %v to a new temp dir: %v", dir, err) {
		return
	}
	_, err = fmt.Fprintf(stdout, "projdir: %v\n", dir)
	contract.IgnoreError(err)

	// Ensure all links are present, the environment is created, and all configs are applied.
	_, err = fmt.Fprintf(opts.Stdout, "Initializing project\n")
	contract.IgnoreError(err)
	runCmd(t, []string{opts.LumiBin, "env", "init", "integrationtesting"}, dir, opts)
	for key, value := range opts.Config {
		runCmd(t, []string{opts.LumiBin, "config", key, value}, dir, opts)
	}

	// Now plan and deploy the real changes.
	_, err = fmt.Fprintf(opts.Stdout, "Performing primary plan and deploy\n")
	contract.IgnoreError(err)
	planAndDeploy := func(d string) {
		runCmd(t, []string{opts.LumiBin, "plan"}, d, opts)
		runCmd(t, []string{opts.LumiBin, "deploy"}, d, opts)
	}
	planAndDeploy(dir)

	// Perform an empty plan and deploy; nothing is expected to happen here.
	_, err = fmt.Fprintf(opts.Stdout, "Performing empty plan and deploy (no changes expected)\n")
	contract.IgnoreError(err)
	planAndDeploy(dir)

	// If there are any edits, apply them and run a plan and deploy for each one.
	for _, edit := range opts.EditDirs {
		_, err = fmt.Fprintf(opts.Stdout, "Applying edit '%v' and rerunning plan and deploy\n", edit)
		contract.IgnoreError(err)
		dir, err = prepareProject(t, edit, dir, opts)
		if !assert.NoError(t, err, "Expected to apply edit %v atop %v, but got an error %v", edit, dir, err) {
			return
		}
		planAndDeploy(dir)
	}

	// Finally, tear down the environment, and clean up the environment.
	_, err = fmt.Fprintf(opts.Stdout, "Destroying environment\n")
	contract.IgnoreError(err)
	runCmd(t, []string{opts.LumiBin, "destroy", "--yes"}, dir, opts)
	runCmd(t, []string{opts.LumiBin, "env", "rm", "--yes", "integrationtesting"}, dir, opts)
}

func runCmd(t *testing.T, args []string, wd string, opts LumiProgramTestOptions) {
	path := args[0]
	command := strings.Join(args, " ")
	var err error
	_, err = fmt.Fprintf(opts.Stdout, "\n**** Invoke '%v' in %v\n", command, wd)
	contract.IgnoreError(err)
	cmd := exec.Cmd{
		Path:   path,
		Dir:    wd,
		Args:   args,
		Stdout: opts.Stdout,
		Stderr: opts.Stderr,
	}
	err = cmd.Run()
	assert.NoError(t, err, "Expected to successfully invoke '%v' in %v: %v", command, wd, err)
}

// prepareProject copies the source directory, srcDir (excluding .lumi), to a new temporary directory.  It then copies
// the .lumi/ directory from the given lumiSrc, if any.  The function returns the newly resulting directory.
func prepareProject(t *testing.T, srcDir string, lumiSrc string, opts LumiProgramTestOptions) (string, error) {
	// Create a new temp directory.
	dir, err := ioutil.TempDir("", "lumi-integration-test-")
	if err != nil {
		return "", err
	}

	// Now copy the source into it, ignoring .lumi.
	if copyerr := copyFile(dir, srcDir, ".lumi"); copyerr != nil {
		return "", copyerr
	}

	// If there's a lumi source directory, copy it atop the target.
	if lumiSrc != "" {
		lumiDir := filepath.Join(lumiSrc, ".lumi")
		if info, err := os.Stat(lumiDir); err == nil && info.IsDir() {
			copyerr := copyFile(filepath.Join(dir, ".lumi"), lumiDir, "")
			if copyerr != nil {
				return "", copyerr
			}
		}
	}

	// Now ensure dependencies are present.
	for _, dependency := range opts.Dependencies {
		runCmd(t, []string{opts.YarnBin, "link", dependency}, dir, opts)
	}

	// And finally compile it.
	runCmd(t, []string{opts.LumiJSBin, "--verbose"}, dir, opts)

	return dir, nil
}

// copyFile is a braindead simple function that copies a src file to a dst file.  Note that it is not general purpose:
// it doesn't handle symbolic links, it doesn't try to be efficient, it doesn't handle copies where src and dst overlap,
// and it makes no attempt to preserve file permissions.  It is what we need for this test package, no more, no less.
func copyFile(dst string, src string, excl string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		// Recursively copy all files in a directory.
		files, err := ioutil.ReadDir(src)
		if err != nil {
			return err
		}
		for _, file := range files {
			name := file.Name()
			if name != excl {
				copyerr := copyFile(filepath.Join(dst, name), filepath.Join(src, name), excl)
				if copyerr != nil {
					return copyerr
				}
			}
		}
	} else if info.Mode().IsRegular() {
		// Copy files by reading and rewriting their contents.  Skip symlinks and other special files.
		data, err := ioutil.ReadFile(src)
		if err != nil {
			return err
		}
		dstdir := filepath.Dir(dst)
		if err = os.MkdirAll(dstdir, 0700); err != nil {
			return err
		}
		if err = ioutil.WriteFile(dst, data, info.Mode()); err != nil {
			return err
		}
	}
	return nil
}

type prefixer struct {
	writer    io.Writer
	prefix    []byte
	anyOutput bool
}

// newPrefixer wraps an io.Writer, prepending a fixed prefix after each \n emitting on the wrapped writer
func newPrefixer(writer io.Writer, prefix string) *prefixer {
	return &prefixer{writer, []byte(prefix), false}
}

var _ io.Writer = (*prefixer)(nil)

func (prefixer *prefixer) Write(p []byte) (int, error) {
	n := 0
	lines := bytes.SplitAfter(p, []byte{'\n'})
	for _, line := range lines {
		if len(line) > 0 {
			_, err := prefixer.writer.Write(prefixer.prefix)
			if err != nil {
				return n, err
			}
		}
		m, err := prefixer.writer.Write(line)
		n += m
		if err != nil {
			return n, err
		}
	}
	return n, nil
}
