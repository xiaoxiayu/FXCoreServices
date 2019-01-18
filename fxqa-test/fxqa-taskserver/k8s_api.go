package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Get node ip from metadata -> labels -> kubernetes.io/hostname
func GetNodes(rest_url string, labels []string) ([]string, error) {
	fmt.Println(rest_url)
	node_ips := []string{}
	for _, label := range labels {
		tem_lable := strings.Split(label, ":")
		if len(tem_lable) != 2 {
			return nil, fmt.Errorf("nodelabel set error.")
		}
	}

	res, err := HTTPGet(rest_url)
	if err != nil {
		return nil, err
	}

	var k8data interface{}
	err = json.Unmarshal(res, &k8data)
	if err != nil {
		return nil, err
	}
	m := k8data.(map[string]interface{})

	for _, v := range m {
		switch vv := v.(type) {
		case []interface{}:
			for _, v1 := range vv {
				vvv := v1.(map[string]interface{})
				metadata_if := vvv["metadata"]
				metadata := metadata_if.(map[string]interface{})

				labels_if := metadata["labels"]
				get_labels := labels_if.(map[string]interface{})
				//				if labels[label_key] != label_val {
				//					continue
				//				}
				b_match := false
				for _, label := range labels {
					label_l := strings.Split(label, ":")
					label_key := label_l[0]
					label_val := label_l[1]
					if get_labels[label_key] == label_val {
						b_match = true
					}
				}
				if !b_match {
					continue
				}
				node_ips = append(node_ips, get_labels["kubernetes.io/hostname"].(string))
			}
		}
	}
	if len(node_ips) == 0 {
		return node_ips, fmt.Errorf("Nodes is empty.")
	}
	return node_ips, nil
}

type K8SPod struct {
	HostIP string
	PodIP  string
}

func GetPods(rest_url string, labels []string) ([]K8SPod, error) {
	fmt.Println(rest_url)
	pods := []K8SPod{}
	for _, label := range labels {
		tem_lable := strings.Split(label, ":")
		if len(tem_lable) != 2 {
			return nil, fmt.Errorf("nodelabel set error.")
		}
	}

	res, err := HTTPGet(rest_url)
	if err != nil {
		return nil, err
	}

	var k8data interface{}
	err = json.Unmarshal(res, &k8data)
	if err != nil {
		return nil, err
	}
	m := k8data.(map[string]interface{})

	for _, v := range m {
		switch vv := v.(type) {
		case []interface{}:
			for _, v1 := range vv {
				//fmt.Println(v1)
				vvv := v1.(map[string]interface{})

				metadata_if := vvv["metadata"]
				metadata := metadata_if.(map[string]interface{})

				labels_if := metadata["labels"]
				get_labels := labels_if.(map[string]interface{})

				b_match := false
				for _, label := range labels {
					label_l := strings.Split(label, ":")
					label_key := label_l[0]
					label_val := label_l[1]
					if get_labels[label_key] == label_val {
						b_match = true
					}
				}
				if !b_match {
					continue
				}
				fmt.Println(get_labels)
				status_if := vvv["status"]
				status := status_if.(map[string]interface{})

				pod := K8SPod{}
				pod.HostIP = status["hostIP"].(string)
				pod.PodIP = status["podIP"].(string)
				pods = append(pods, pod)
			}
		}
	}
	if len(pods) == 0 {
		return pods, fmt.Errorf("Pods is empty.")
	}
	return pods, nil
}
