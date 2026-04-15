//go:build ignore

package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	db, err := sql.Open("sqlite3", "./data/miao.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// 图片映射到任务ID
	materials := []struct {
		TaskID    int
		FileName  string
		FilePath  string
		FileType  string
		SortOrder int
	}{
		{1, "task1_drinks.jpg", "/static/uploads/image/task1_drinks.jpg", "image", 1},
		{2, "task2_fashion.jpg", "/static/uploads/image/task2_fashion.jpg", "image", 1},
		{3, "task3_earphones.jpg", "/static/uploads/image/task3_earphones.jpg", "image", 1},
		{4, "task4_restaurant.jpg", "/static/uploads/image/task4_restaurant.jpg", "image", 1},
		{5, "task5_travel.jpg", "/static/uploads/image/task5_travel.jpg", "image", 1},
		{6, "task6_makeup.jpg", "/static/uploads/image/task6_makeup.jpg", "image", 1},
		{7, "task7_workout.jpg", "/static/uploads/image/task7_workout.jpg", "image", 1},
		{8, "task8_books.jpg", "/static/uploads/image/task8_books.jpg", "image", 1},
		{9, "task9_tablet.jpg", "/static/uploads/image/task9_tablet.jpg", "image", 1},
		{10, "task10_home.jpg", "/static/uploads/image/task10_home.jpg", "image", 1},
	}

	now := time.Now()

	for _, m := range materials {
		// 获取文件大小
		absPath := filepath.Join("/data/miao/web", m.FilePath)
		info, err := os.Stat(absPath)
		var fileSize int64
		if err == nil {
			fileSize = info.Size()
		}

		_, err = db.Exec(`
			INSERT INTO task_materials (
				task_id, file_name, file_path, file_size, file_type, sort_order, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			m.TaskID, m.FileName, m.FilePath, fileSize, m.FileType, m.SortOrder, now,
		)
		if err != nil {
			fmt.Printf("❌ 添加素材失败 [task_id=%d]: %v\n", m.TaskID, err)
			continue
		}
		fmt.Printf("✅ 素材添加成功: task_id=%d, file=%s\n", m.TaskID, m.FileName)
	}

	fmt.Printf("\n🎉 共添加 %d 个素材！\n", len(materials))
}