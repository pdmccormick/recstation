package recstation

import (
	"os"
	"os/exec"
)

type CmdExit struct {
	Cmd *exec.Cmd
	Err error
}

func RunAndReportCmd(cmd *exec.Cmd, report chan CmdExit) {
	go func() {
		err := cmd.Run()
		report <- CmdExit{
			Cmd: cmd,
			Err: err,
		}
	}()
}

func PipeCmds(left *exec.Cmd, right *exec.Cmd) error {
	pr, pw, err := os.Pipe()
	if err != nil {
		return err
	}

	left.Stdout = pw
	right.Stdin = pr

	return nil
}
