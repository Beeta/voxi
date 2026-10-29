//go:build !darwin

package app

import "errors"

// freeDisk is unavailable off macOS; recording is too.
func freeDisk(string) (int64, error) { return 0, errors.New("磁盘空间检查仅支持 macOS") }
