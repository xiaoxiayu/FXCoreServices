package main

import (
	"fmt"
	"os"
	//	"path"
	"io/ioutil"
	"net/http"
	"path/filepath"
)

func GetFileSize(file string) int64 {
	f, e := os.Stat(file)
	if e != nil {
		return 0
	}
	return f.Size()
}

func SubString(str string, start, length int) (substr string) {
	rs := []rune(str)
	rl := len(rs)
	end := 0

	if start < 0 {
		start = rl - 1 + start
	}
	end = start + length

	if start > end {
		start, end = end, start
	}

	if start < 0 {
		start = 0
	}
	if start > rl {
		start = rl
	}
	if end < 0 {
		end = 0
	}
	if end > rl {
		end = rl
	}

	return string(rs[start:end])
}

func isFileOrDir(filename string, decideDir bool) bool {
	fileInfo, err := os.Stat(filename)
	if err != nil {
		return false
	}
	isDir := fileInfo.IsDir()
	if decideDir {
		return isDir
	}
	return !isDir
}

func IsDir(filename string) bool {
	return isFileOrDir(filename, true)
}

func Exist(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil || os.IsExist(err)
}

func GetFilelist(path_str string, w_fp *os.File) []string {
	file_list := []string{}
	err := filepath.Walk(path_str, func(path_str string, f os.FileInfo, err error) error {
		if f == nil {
			return err
		}
		if f.IsDir() {
			return nil
		}
		file_list = append(file_list, path_str)
		return nil
	})
	if err != nil {
		fmt.Printf("filepath.Walk() returned %v\n", err)
	}
	return file_list
}

func HTTPGet(url_str string) ([]byte, error) {
	client := http.Client{
		Timeout: 6000e9,
	}
	r, err := client.Get(url_str)
	if err != nil {
		return nil, err
	}

	defer r.Body.Close()

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	return body, err
}
