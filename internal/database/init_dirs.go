package database

import (
	"log"
	"os"
	"path/filepath"
)

// InitDirectories 初始化必要的目录结构
func InitDirectories() error {
	dirs := []string{
		"data",
		"web/static/uploads/image",
		"web/static/uploads/video",
		"logs",
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		log.Printf("✓ 目录已就绪: %s", dir)
	}

	// 创建 .gitkeep 文件保持目录结构
	gitkeepFiles := []string{
		"web/static/uploads/image/.gitkeep",
		"web/static/uploads/video/.gitkeep",
	}

	for _, file := range gitkeepFiles {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			if err := os.WriteFile(file, []byte(""), 0644); err != nil {
				log.Printf("警告: 无法创建 %s: %v", file, err)
			}
		}
	}

	return nil
}

// CleanupOldUploads 清理旧的上传文件（可选，在启动时调用）
func CleanupOldUploads(maxAgeDays int) error {
	// 这个函数可以在后台定期运行
	// 暂时留空，由独立脚本处理
	return nil
}

// GetUploadStats 获取上传目录统计信息
func GetUploadStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	uploadDir := "web/static/uploads"
	var totalSize int64
	var fileCount int

	err := filepath.Walk(uploadDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 忽略错误继续
		}
		if !info.IsDir() {
			totalSize += info.Size()
			fileCount++
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	stats["total_files"] = fileCount
	stats["total_size_mb"] = float64(totalSize) / 1024 / 1024
	stats["upload_dir"] = uploadDir

	return stats, nil
}
