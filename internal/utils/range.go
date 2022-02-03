package utils

import (
	"fmt"
	"strconv"
	"strings"
)

func ParseRange(spec string) (int64, int64, error) {
	if spec == "" {
		return 0, 0, fmt.Errorf("no range specified")
	}
	parts := strings.Split(spec, "-")
	if len(parts) == 1 {
		h, err := strconv.ParseInt(parts[0], 10, 64)
		return -1, h, err
	}
	l, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, 0, err
	}
	if parts[1] == "" {
		return l, -1, nil
	}
	h, err := strconv.ParseInt(parts[1], 10, 64)
	return l, h, err
}
