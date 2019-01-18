package main

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

	cmd_str := fmt.Sprintf("taskkill /F /T /pid %d", pid)
	cmd_list := strings.Split(cmd_str, " ")

	cmd := exec.Command(cmd_list[0], cmd_list[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}
