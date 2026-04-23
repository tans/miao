package database

import (
	"log"
	"os"
)

// InitDirectories 初始化必要的目录结构
func InitDirectories() error {
	dirs := []string{
		"data",
		"logs",
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		log.Printf("✓ 目录已就绪: %s", dir)
	}

	return nil
}

// CleanupOldUploads 清理旧的上传文件（可选，在启动时调用）
func CleanupOldUploads(maxAgeDays int) error {
	// 这个函数可以在后台定期运行
	// 暂时留空，由独立脚本处理
	return nil
}

// GetUploadStats 获取上传目录统计信息（已迁移到 S3，此函数仅作兼容）
func GetUploadStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})
	stats["total_files"] = 0
	stats["total_size_mb"] = 0
	stats["upload_dir"] = "已迁移到 S3"
	return stats, nil
}
