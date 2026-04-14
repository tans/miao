package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	db, err := sql.Open("sqlite3", "./data/miao.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// 确保business_id=1的用户有足够余额
	_, err = db.Exec("UPDATE users SET balance = 100000 WHERE id = 1")
	if err != nil {
		log.Printf("Warning: failed to update balance: %v", err)
	}

	// 10个虚构任务数据
	tasks := []struct {
		Title          string
		Description    string
		Industries     string
		VideoDuration  string
		VideoAspect    string
		VideoResolution string
		CreativeStyle  string
		UnitPrice      float64
		AwardPrice     float64
		AwardCount     int
		TotalCount     int
	}{
		{
			Title:           "夏日饮品推荐短视频创作",
			Description:     "创作一条15-60秒的短视频，展示夏日饮品制作过程或品尝体验，要求画面清晰，有字幕说明，背景音乐轻快活泼。",
			Industries:      "餐饮,食品",
			VideoDuration:   "60秒内",
			VideoAspect:     "9:16",
			VideoResolution: "1080P",
			CreativeStyle:   "搞笑轻松",
			UnitPrice:       30,
			AwardPrice:      100,
			AwardCount:      3,
			TotalCount:      50,
		},
		{
			Title:           "春季穿搭分享小红书笔记",
			Description:     "撰写一篇图文笔记，介绍春季流行穿搭搭配，包含至少3套搭配方案，图片要求清晰美观，有搭配说明。",
			Industries:      "时尚,服装",
			VideoDuration:   "不限制",
			VideoAspect:     "1:1",
			VideoResolution: "1080P",
			CreativeStyle:   "种草安利",
			UnitPrice:       25,
			AwardPrice:      80,
			AwardCount:      5,
			TotalCount:      30,
		},
		{
			Title:           "蓝牙耳机深度测评视频",
			Description:     "对指定蓝牙耳机进行深度测评，包含外观、音质、续航、佩戴舒适度等方面，时长3-5分钟，口播清晰专业。",
			Industries:      "数码,电子产品",
			VideoDuration:   "1-3分钟",
			VideoAspect:     "16:9",
			VideoResolution: "1080P",
			CreativeStyle:   "科普专业",
			UnitPrice:       50,
			AwardPrice:      200,
			AwardCount:      2,
			TotalCount:      20,
		},
		{
			Title:           "本地网红餐厅探店图文",
			Description:     "到指定餐厅进行探店拍摄，产出图文笔记一份，分享真实用餐体验，需包含环境、菜品、价位等关键信息。",
			Industries:      "餐饮,美食",
			VideoDuration:   "不限制",
			VideoAspect:     "1:1",
			VideoResolution: "720P",
			CreativeStyle:   "种草安利",
			UnitPrice:       35,
			AwardPrice:      120,
			AwardCount:      4,
			TotalCount:      40,
		},
		{
			Title:           "周末周边游旅游攻略",
			Description:     "规划一条2天1夜的周边游路线，包含景点、美食、住宿推荐，需包含详细费用预算，图文并茂。",
			Industries:      "旅游,出行",
			VideoDuration:   "不限制",
			VideoAspect:     "9:16",
			VideoResolution: "1080P",
			CreativeStyle:   "温情故事",
			UnitPrice:       40,
			AwardPrice:      150,
			AwardCount:      3,
			TotalCount:      25,
		},
		{
			Title:           "日常通勤妆容教程视频",
			Description:     "录制一个日常通勤妆容教程视频，时长5-10分钟，讲解妆容步骤和产品推荐，化妆技巧实用易学。",
			Industries:      "美妆,个护",
			VideoDuration:   "1-3分钟",
			VideoAspect:     "9:16",
			VideoResolution: "1080P",
			CreativeStyle:   "种草安利",
			UnitPrice:       45,
			AwardPrice:      180,
			AwardCount:      3,
			TotalCount:      30,
		},
		{
			Title:           "居家运动计划跟练视频",
			Description:     "制定一份居家运动计划，包含热身、力量训练、拉伸三个部分，需拍摄运动过程视频，动作标准有示范。",
			Industries:      "运动,健康",
			VideoDuration:   "1-3分钟",
			VideoAspect:     "9:16",
			VideoResolution: "1080P",
			CreativeStyle:   "口语化",
			UnitPrice:       28,
			AwardPrice:      90,
			AwardCount:      5,
			TotalCount:      45,
		},
		{
			Title:           "每周书单推荐分享笔记",
			Description:     "分享本周阅读的书籍，写一篇500字以上的读书笔记，需包含书籍封面图和精彩片段摘录，有个人感悟。",
			Industries:      "教育,图书",
			VideoDuration:   "不限制",
			VideoAspect:     "1:1",
			VideoResolution: "720P",
			CreativeStyle:   "温情故事",
			UnitPrice:       20,
			AwardPrice:      60,
			AwardCount:      5,
			TotalCount:      60,
		},
		{
			Title:           "平板电脑横评对比视频",
			Description:     "对两款平板电脑进行横评对比，产出视频和图文两版内容，包含性能测试、屏幕对比、续航测试等客观数据。",
			Industries:      "数码,电子产品",
			VideoDuration:   "1-3分钟",
			VideoAspect:     "16:9",
			VideoResolution: "1080P",
			CreativeStyle:   "科普专业",
			UnitPrice:       55,
			AwardPrice:      250,
			AwardCount:      2,
			TotalCount:      15,
		},
		{
			Title:           "家居好物开箱分享视频",
			Description:     "对指定家居产品进行开箱视频创作，展示产品外观、功能、使用场景，需有真实使用体验分享。",
			Industries:      "家居,生活",
			VideoDuration:   "60秒内",
			VideoAspect:     "9:16",
			VideoResolution: "1080P",
			CreativeStyle:   "搞笑轻松",
			UnitPrice:       32,
			AwardPrice:      100,
			AwardCount:      4,
			TotalCount:      35,
		},
	}

	now := time.Now()
	endDate := now.AddDate(0, 0, 14) // 14天后截止

	for i, t := range tasks {
		totalBudget := t.UnitPrice*float64(t.TotalCount) + t.AwardPrice*float64(t.AwardCount)
		remainingCount := t.TotalCount

		result, err := db.Exec(`
			INSERT INTO tasks (
				business_id, title, description, category, unit_price, total_count,
				remaining_count, status, total_budget, frozen_amount, paid_amount,
				created_at, updated_at, industries, video_duration, video_aspect,
				video_resolution, creative_style, award_price, award_count,
				publish_at, end_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			1, t.Title, t.Description, 3, t.UnitPrice, t.TotalCount,
			remainingCount, 2, totalBudget, 0, 0,
			now, now, t.Industries, t.VideoDuration, t.VideoAspect,
			t.VideoResolution, t.CreativeStyle, t.AwardPrice, t.AwardCount,
			now, endDate,
		)
		if err != nil {
			fmt.Printf("❌ 创建任务失败 [%d]: %v\n", i+1, err)
			continue
		}

		taskID, _ := result.LastInsertId()
		fmt.Printf("✅ 任务 [%d/%d] 创建成功: %s (ID: %d, 预算: %.0f元)\n", i+1, len(tasks), t.Title, taskID, totalBudget)
	}

	fmt.Printf("\n🎉 共创建 %d 个虚构任务数据！\n", len(tasks))
}