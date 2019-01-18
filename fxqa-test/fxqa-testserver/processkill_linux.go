package main

import (
	//	"fmt"
	//	"sort"
	"strconv"
	//	"strings"
	"syscall"
)

type PidInfo struct {
	pid int
	pri int
}

func Kill(pid_str string, signal syscall.Signal) error {
	pid, _ := strconv.Atoi(pid_str)
	err := syscall.Kill(pid, syscall.SIGKILL)
	if err != nil {
		return err
	}

	return nil
}
