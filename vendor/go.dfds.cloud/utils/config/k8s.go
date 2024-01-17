package config

import "os"

const SA_TOKEN_PATH = "/var/run/secrets/kubernetes.io/serviceaccount"

func GetInClusterK8sToken() (string, error) {
	_, err := os.Stat(SA_TOKEN_PATH)
	if err != nil {
		return "", err
	}

	dat, err := os.ReadFile(SA_TOKEN_PATH)
	if err != nil {
		return "", err
	}

	return string(dat), nil
}