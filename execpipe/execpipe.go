package execpipe

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"

	"github.com/goaux/stacktrace/v2"
)

// CheckPath checks if the given executable exists in the system's PATH.
// It returns an error if the executable is not found, or nil if it is.
func CheckPath(executable string) error {
	_, err := stacktrace.Trace2(exec.LookPath(executable))
	return err
}

// Run executes an external command with the given arguments, piping its stdout
// to the provided writer and capturing its stderr.
//
// Parameters:
//
//	w: The io.Writer to which the command's stdout will be piped.
//	r: The io.Reader from which the command's stdin will be read.
//	name: The name of the executable to run.
//	args: Variable number of strings representing the arguments to pass to the executable.
//
// Returns:
//
//	An error if the command fails to execute or if there's an issue with the piping.
//	The error message includes the command name, the underlying error, and the captured stderr.
func Run(w io.Writer, r io.Reader, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = r
	if out, err := cmd.StdoutPipe(); err != nil {
		return err
	} else {
		go io.Copy(w, out)
	}
	stderr := new(bytes.Buffer)
	cmd.Stderr = stderr
	if err := stacktrace.Trace(cmd.Run()); err != nil {
		return fmt.Errorf("error: %s, cause=%w, stderr=%q", name, err, stderr.String())
	}
	return nil
}
