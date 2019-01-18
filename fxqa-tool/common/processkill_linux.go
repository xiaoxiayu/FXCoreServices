package common

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

type PidInfo struct {
	pid int
	pri int
}

func Kill(pid_str string, signal syscall.Signal) error {
	pid, _ := strconv.Atoi(pid_str)
	err := syscall.Kill(pid, signal)
	if err != nil {
		return err
	}

	return nil
}

func KillByName(process_name string) error {
	cmd_str := fmt.Sprintf("pkill %s", process_name)
	cmd_list := strings.Split(cmd_str, " ")

	cmd := exec.Command(cmd_list[0], cmd_list[1:]...)
	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}
