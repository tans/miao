//go:build ignore

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// 清理超过指定天数的上传文件
func main() {
	uploadDir := "web/static/uploads"
	maxAge := 7 * 24 * time.Hour // 7天

	if len(os.Args) > 1 {
		days := 7
		fmt.Sscanf(os.Args[1], "%d", &days)
		maxAge = time.Duration(days) * 24 * time.Hour
	}

	fmt.Printf("清理 %s 目录中超过 %v 的文件...\n", uploadDir, maxAge)

	var deletedCount int
	var deletedSize int64

	err := filepath.Walk(uploadDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过目录
		if info.IsDir() {
			return nil
		}

		// 检查文件年龄
		age := time.Since(info.ModTime())
		if age > maxAge {
			fmt.Printf("删除: %s (%.2f MB, %v 前)\n", path, float64(info.Size())/1024/1024, age.Round(time.Hour))

			if err := os.Remove(path); err != nil {
				fmt.Printf("  错误: %v\n", err)
				return nil
			}

			deletedCount++
			deletedSize += info.Size()
		}

		return nil
	})

	if err != nil {
		fmt.Printf("错误: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n清理完成:\n")
	fmt.Printf("  删除文件数: %d\n", deletedCount)
	fmt.Printf("  释放空间: %.2f MB\n", float64(deletedSize)/1024/1024)
}
