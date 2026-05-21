package cmd

import (
	"bytes"
	"log"
	"os/exec"
	"strings"
)

// Empty netNSPath means no net ns.
func Exec(netNSPath string, verbose bool, cmd string, cmdArgs []string) ([]byte, error) {
	var (
		execArgs []string
		execPath string
	)
	if len(netNSPath) != 0 {
		execPath = "nsenter"
		execArgs = []string{"--net=" + netNSPath, "--", cmd}
		execArgs = append(execArgs, cmdArgs...)
	} else {
		execPath = cmd
		execArgs = cmdArgs
	}
	if verbose {
		log.Printf("[#] %s %s", execPath, strings.Join(execArgs, " "))
	}

	var stdout, stderr bytes.Buffer
	execCmd := exec.Command(execPath, execArgs...)
	execCmd.Stdout = &stdout
	execCmd.Stderr = &stderr
	if err := execCmd.Run(); err != nil {
		log.Printf("%s failed: %s: %s", cmd, err, stderr.Bytes())
		return nil, err
	}

	return stdout.Bytes(), nil
}
