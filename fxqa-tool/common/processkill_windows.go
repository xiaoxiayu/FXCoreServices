package common

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

func Kill(pid_str string, signal syscall.Signal) error {
	pid, _ := strconv.Atoi(pid_str)

	cmd_str := fmt.Sprintf("taskkill /F /pid %d", pid)
	cmd_list := strings.Split(cmd_str, " ")

	cmd := exec.Command(cmd_list[0], cmd_list[1:]...)
	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func KillByName(process_name string) error {
	cmd_str := fmt.Sprintf("taskkill /f /im %s", process_name)
	cmd_list := strings.Split(cmd_str, " ")

	cmd := exec.Command(cmd_list[0], cmd_list[1:]...)
	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}
