package config

import (
	"os"
	"strconv"
	"strings"
)

func GetEnvValue(key string, def string) string {
	val := os.Getenv(key)
	if len(val) == 0 {
		return def
	}

	return val
}

func GetEnvInt(key string, def int) (int, error) {
	val := os.Getenv(key)
	if len(val) == 0 {
		return def, nil
	}

	payload, err := strconv.Atoi(val)
	if err != nil {
		return 0, err
	}

	return payload, nil
}

func GetEnvInt64(key string, def int64) (int64, error) {
	val := os.Getenv(key)
	if len(val) == 0 {
		return def, nil
	}

	payload, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return 0, err
	}

	return payload, nil
}

func GetEnvInt32(key string, def int32) (int32, error) {
	val := os.Getenv(key)
	if len(val) == 0 {
		return def, nil
	}

	payload, err := strconv.ParseInt(val, 10, 32)
	if err != nil {
		return 0, err
	}

	return int32(payload), nil
}

func GetEnvBool(key string, def bool) bool {
	val := os.Getenv(key)

	if len(val) == 0 {
		return def
	}

	if strings.Compare("false", strings.ToLower(val)) == 0 {
		return false
	}

	if strings.Compare("true", strings.ToLower(val)) == 0 {
		return true
	}

	return def
}
