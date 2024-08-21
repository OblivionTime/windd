package utils

import (
	"fmt"
	"os"
)

// formatFileSize 将文件大小转换为易读的格式
func FormatFileSize(size uint64) string {
	// 定义文件大小单位
	units := []string{"B", "KB", "MB", "GB", "TB", "PB"}

	// 处理文件大小为0的情况
	if size == 0 {
		return "0 B"
	}

	// 计算文件大小所在单位的索引
	unitIndex := 0
	for size >= 1024 && unitIndex < len(units)-1 {
		size /= 1024
		unitIndex++
	}

	// 格式化文件大小
	return fmt.Sprintf("%d%s", size, units[unitIndex])
}

// 判断当前文件是否存在
func IsExist(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsExist(err) {
			return true
		}
		if os.IsNotExist(err) {
			return false
		}
		return false
	}
	return true
}

// 获取文件大小
func CalculateFileSize(path string) uint64 {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return 0
	}
	fileSize := fileInfo.Size()
	return uint64(fileSize)
}
