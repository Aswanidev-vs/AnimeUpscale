//go:build !windows || !cgo

package nativeexec

import "os/exec"

func Run(binary string, args []string) ([]byte, error) {
	cmd := exec.Command(binary, args...)
	return cmd.CombinedOutput()
}
