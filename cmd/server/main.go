package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/joho/godotenv"
	"github.com/tans/miao/internal/config"
	"github.com/tans/miao/internal/database"
	"github.com/tans/miao/internal/router"
)

func main() {
	// Load .env file if exists
	_ = godotenv.Load()

	// Load configuration
	cfg := config.Load()

	// Initialize directories
	if err := database.InitDirectories(); err != nil {
		log.Fatalf("Failed to initialize directories: %v", err)
	}

	// Initialize database
	db, err := database.InitDB(cfg.Database.Path)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()
	log.Printf("Database: %s", cfg.Database.Path)

	// Run migrations
	if err := database.RunAllMigrations(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
	log.Println("Database migrations completed successfully")

	// Start background workers
	go startBackgroundWorkers(db)

	// Setup router
	r := router.SetupRouter()

	// Create HTTP server
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Server.Port),
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Server starting on port %s", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// Give outstanding requests 5 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

// startBackgroundWorkers 启动后台定时任务
// 优化：每5分钟执行一次，减少资源消耗
func startBackgroundWorkers(db *sql.DB) {
	log.Println("Background workers starting...")
	ticker := time.NewTicker(5 * time.Minute)
	quit := make(chan struct{})

	go func() {
		log.Println("Background workers ticker started")
		for {
			select {
			case <-ticker.C:
				log.Println("Background workers: running checks...")
				checkExpiredClaims(db)
				checkExpiredReviews(db)
				checkExpiredTasks(db)
				resetDailyClaimCount(db)
			case <-quit:
				ticker.Stop()
				log.Println("Background workers stopped")
				return
			}
		}
	}()
}

// checkExpiredClaims 检查认领超时（24h未提交）
func checkExpiredClaims(db *sql.DB) {
	now := time.Now()

	// 查找已认领但超时的 claims (status=1, expires_at < now)
	rows, err := db.Query(`
		SELECT c.id, c.task_id, c.creator_id, t.remaining_count, u.margin_frozen
		FROM claims c
		JOIN tasks t ON c.task_id = t.id
		JOIN users u ON c.creator_id = u.id
		WHERE c.status = 1 AND c.expires_at < ?
	`, now)
	if err != nil {
		log.Printf("checkExpiredClaims query error: %v", err)
		return
	}
	defer rows.Close()

	type expiredClaim struct {
		claimID         int
		taskID          int
		creatorID       int
		remainingCount  int
		marginFrozen    float64
	}

	var claims []expiredClaim
	for rows.Next() {
		var c expiredClaim
		if err := rows.Scan(&c.claimID, &c.taskID, &c.creatorID, &c.remainingCount, &c.marginFrozen); err != nil {
			log.Printf("checkExpiredClaims scan error: %v", err)
			continue
		}
		claims = append(claims, c)
	}

	for _, c := range claims {
		tx, err := db.Begin()
		if err != nil {
			log.Printf("checkExpiredClaims begin tx error: %v", err)
			continue
		}

		// 标记为超时 status=5
		_, err = tx.Exec(`UPDATE claims SET status = 5, updated_at = ? WHERE id = ?`, now, c.claimID)
		if err != nil {
			tx.Rollback()
			log.Printf("checkExpiredClaims update claim status error: %v", err)
			continue
		}

		// 归还任务 remaining_count
		_, err = tx.Exec(`UPDATE tasks SET remaining_count = remaining_count + 1 WHERE id = ?`, c.taskID)
		if err != nil {
			tx.Rollback()
			log.Printf("checkExpiredClaims update task remaining_count error: %v", err)
			continue
		}

		// 退还保证金给创作者
		if c.marginFrozen > 0 {
			_, err = tx.Exec(`UPDATE users SET margin_frozen = 0, balance = balance + ? WHERE id = ?`, c.marginFrozen, c.creatorID)
			if err != nil {
				tx.Rollback()
				log.Printf("checkExpiredClaims update user margin error: %v", err)
				continue
			}

			// 记录交易
			_, err = tx.Exec(`
				INSERT INTO transactions (user_id, type, amount, balance_before, balance_after, remark, related_id, created_at)
				VALUES (?, 6, ?, (SELECT balance FROM users WHERE id = ?) - ?, (SELECT balance FROM users WHERE id = ?), '认领超时退还保证金', ?, ?)
			`, c.creatorID, c.marginFrozen, c.creatorID, c.marginFrozen, c.creatorID, c.claimID, now)
			if err != nil {
				tx.Rollback()
				log.Printf("checkExpiredClaims insert transaction error: %v", err)
				continue
			}
		}

		if err := tx.Commit(); err != nil {
			log.Printf("checkExpiredClaims commit error: %v", err)
			continue
		}

		log.Printf("checkExpiredClaims: claim %d timeout, returned margin %.2f to creator %d", c.claimID, c.marginFrozen, c.creatorID)
	}
}

	// checkExpiredReviews 检查验收超时（48h未验收 -> 自动通过）
func checkExpiredReviews(db *sql.DB) {
	now := time.Now()

	// 查找已提交但超时的 claims (status=2, submit_at + 48h < now)
	rows, err := db.Query(`
		SELECT c.id, c.task_id, c.creator_id, t.unit_price, c.creator_reward, c.margin_returned, u.margin_frozen, u.level
		FROM claims c
		JOIN tasks t ON c.task_id = t.id
		JOIN users u ON c.creator_id = u.id
		WHERE c.status = 2 AND datetime(c.submit_at, '+48 hours') < ?
	`, now)
	if err != nil {
		log.Printf("checkExpiredReviews query error: %v", err)
		return
	}
	defer rows.Close()

	type expiredReview struct {
		claimID         int
		taskID          int
		creatorID       int
		unitPrice       float64
		creatorReward   float64
		marginReturned  float64
		marginFrozen    float64
		creatorLevel    int
	}

	var reviews []expiredReview
	for rows.Next() {
		var r expiredReview
		if err := rows.Scan(&r.claimID, &r.taskID, &r.creatorID, &r.unitPrice, &r.creatorReward, &r.marginReturned, &r.marginFrozen, &r.creatorLevel); err != nil {
			log.Printf("checkExpiredReviews scan error: %v", err)
			continue
		}
		reviews = append(reviews, r)
	}

	for _, r := range reviews {
		tx, err := db.Begin()
		if err != nil {
			log.Printf("checkExpiredReviews begin tx error: %v", err)
			continue
		}

		// 自动通过：status=3, review_result=1
		_, err = tx.Exec(`UPDATE claims SET status = 3, review_result = 1, review_at = ?, updated_at = ? WHERE id = ?`, now, now, r.claimID)
		if err != nil {
			tx.Rollback()
			log.Printf("checkExpiredReviews update claim status error: %v", err)
			continue
		}

		// 根据创作者等级计算动态抽成
		var commissionRate float64
		switch r.creatorLevel {
		case 4: // 钻石
			commissionRate = 0.10
		case 3: // 黄金
			commissionRate = 0.12
		case 2: // 白银
			commissionRate = 0.15
		default: // 青铜
			commissionRate = 0.20
		}

		creatorReward := r.unitPrice * (1.0 - commissionRate)
		platformFee := r.unitPrice * commissionRate
		_ = platformFee // TODO: 记录平台收入

		// 更新创作者余额和保证金
		_, err = tx.Exec(`
			UPDATE users
			SET balance = balance + ?, margin_frozen = margin_frozen - ?
			WHERE id = ?
		`, creatorReward, r.marginFrozen, r.creatorID)
		if err != nil {
			tx.Rollback()
			log.Printf("checkExpiredReviews update creator balance error: %v", err)
			continue
		}

		// 记录创作者收入交易
		_, err = tx.Exec(`
			INSERT INTO transactions (user_id, type, amount, balance_before, balance_after, remark, related_id, created_at)
			VALUES (?, 4, ?, (SELECT balance FROM users WHERE id = ?) - ?, (SELECT balance FROM users WHERE id = ?), '任务通过结算', ?, ?)
		`, r.creatorID, creatorReward, r.creatorID, creatorReward, r.creatorID, r.claimID, now)
		if err != nil {
			tx.Rollback()
			log.Printf("checkExpiredReviews insert creator transaction error: %v", err)
			continue
		}

		// 更新任务 paid_amount
		_, err = tx.Exec(`UPDATE tasks SET paid_amount = paid_amount + ? WHERE id = ?`, r.unitPrice, r.taskID)
		if err != nil {
			tx.Rollback()
			log.Printf("checkExpiredReviews update task paid_amount error: %v", err)
			continue
		}

		// 退还保证金
		if r.marginFrozen > 0 {
			_, err = tx.Exec(`UPDATE users SET margin_frozen = margin_frozen - ? WHERE id = ?`, r.marginFrozen, r.creatorID)
			if err != nil {
				tx.Rollback()
				log.Printf("checkExpiredReviews update margin error: %v", err)
				continue
			}

			// 记录保证金退还交易
			_, err = tx.Exec(`
				INSERT INTO transactions (user_id, type, amount, balance_before, balance_after, remark, related_id, created_at)
				VALUES (?, 7, ?, (SELECT balance FROM users WHERE id = ?) - ?, (SELECT balance FROM users WHERE id = ?), '保证金退还', ?, ?)
			`, r.creatorID, r.marginFrozen, r.creatorID, r.marginFrozen, r.creatorID, r.claimID, now)
			if err != nil {
				tx.Rollback()
				log.Printf("checkExpiredReviews insert margin transaction error: %v", err)
				continue
			}
		}

		if err := tx.Commit(); err != nil {
			log.Printf("checkExpiredReviews commit error: %v", err)
			continue
		}

		log.Printf("checkExpiredReviews: claim %d auto passed, reward %.2f to creator %d (level %d, commission %.0f%%)", r.claimID, creatorReward, r.creatorID, r.creatorLevel, commissionRate*100)
	}
}

// checkExpiredTasks 检查任务超时（到达截止时间）
func checkExpiredTasks(db *sql.DB) {
	now := time.Now()

	// 查找已到期但仍在进行中的任务 (status=2或3, end_at < now)
	rows, err := db.Query(`
		SELECT t.id, t.business_id, t.title, t.total_budget, t.frozen_amount, t.paid_amount, t.remaining_count
		FROM tasks t
		WHERE t.status IN (2, 3) AND t.end_at IS NOT NULL AND t.end_at < ?
	`, now)
	if err != nil {
		log.Printf("checkExpiredTasks query error: %v", err)
		return
	}
	defer rows.Close()

	type expiredTask struct {
		taskID          int
		businessID      int
		title           string
		totalBudget     float64
		frozenAmount    float64
		paidAmount      float64
		remainingCount  int
	}

	var tasks []expiredTask
	for rows.Next() {
		var t expiredTask
		if err := rows.Scan(&t.taskID, &t.businessID, &t.title, &t.totalBudget, &t.frozenAmount, &t.paidAmount, &t.remainingCount); err != nil {
			log.Printf("checkExpiredTasks scan error: %v", err)
			continue
		}
		tasks = append(tasks, t)
	}

	for _, t := range tasks {
		tx, err := db.Begin()
		if err != nil {
			log.Printf("checkExpiredTasks begin tx error: %v", err)
			continue
		}

		// 计算应返还的冻结金额
		unfrozenAmount := t.frozenAmount - t.paidAmount

		// 返还冻结金额给商家
		if unfrozenAmount > 0 {
			_, err = tx.Exec(`
				UPDATE users
				SET balance = balance + ?, frozen_amount = frozen_amount - ?
				WHERE id = ?
			`, unfrozenAmount, unfrozenAmount, t.businessID)
			if err != nil {
				tx.Rollback()
				log.Printf("checkExpiredTasks update business balance error: %v", err)
				continue
			}

			// 记录交易
			_, err = tx.Exec(`
				INSERT INTO transactions (user_id, type, amount, balance_before, balance_after, remark, related_id, created_at)
				VALUES (?, 8, ?, (SELECT balance FROM users WHERE id = ?) - ?, (SELECT balance FROM users WHERE id = ?), ?, ?, ?)
			`, t.businessID, unfrozenAmount, t.businessID, unfrozenAmount, t.businessID, "任务结束解冻: "+t.title, t.taskID, now)
			if err != nil {
				tx.Rollback()
				log.Printf("checkExpiredTasks insert transaction error: %v", err)
				continue
			}
		}

		// 标记任务为已结束 (status=4)
		_, err = tx.Exec(`UPDATE tasks SET status = 4, updated_at = ? WHERE id = ?`, now, t.taskID)
		if err != nil {
			tx.Rollback()
			log.Printf("checkExpiredTasks update task status error: %v", err)
			continue
		}

		// 取消所有待提交的认领 (status=1) 并退还保证金
		claimRows, err := tx.Query(`
			SELECT c.id, c.creator_id, u.margin_frozen
			FROM claims c
			JOIN users u ON c.creator_id = u.id
			WHERE c.task_id = ? AND c.status = 1
		`, t.taskID)
		if err != nil {
			tx.Rollback()
			log.Printf("checkExpiredTasks query claims error: %v", err)
			continue
		}

		var claimsToCancel []struct{ claimID, creatorID int; marginFrozen float64 }
		for claimRows.Next() {
			var c struct{ claimID, creatorID int; marginFrozen float64 }
			if err := claimRows.Scan(&c.claimID, &c.creatorID, &c.marginFrozen); err != nil {
				continue
			}
			claimsToCancel = append(claimsToCancel, c)
		}
		claimRows.Close()

		for _, c := range claimsToCancel {
			// 取消认领
			_, err = tx.Exec(`UPDATE claims SET status = 4, updated_at = ? WHERE id = ?`, now, c.claimID)
			if err != nil {
				log.Printf("checkExpiredTasks update claim status error: %v", err)
				continue
			}

			// 退还保证金
			if c.marginFrozen > 0 {
				_, err = tx.Exec(`
					UPDATE users SET margin_frozen = 0, balance = balance + ? WHERE id = ?
				`, c.marginFrozen, c.creatorID)
				if err != nil {
					log.Printf("checkExpiredTasks update creator margin error: %v", err)
					continue
				}

				// 记录交易
				_, err = tx.Exec(`
					INSERT INTO transactions (user_id, type, amount, balance_before, balance_after, remark, related_id, created_at)
					VALUES (?, 9, ?, (SELECT balance FROM users WHERE id = ?) - ?, (SELECT balance FROM users WHERE id = ?), '任务取消退还保证金', ?, ?)
				`, c.creatorID, c.marginFrozen, c.creatorID, c.marginFrozen, c.creatorID, c.claimID, now)
				if err != nil {
					log.Printf("checkExpiredTasks insert margin transaction error: %v", err)
				}
			}
		}

		if err := tx.Commit(); err != nil {
			log.Printf("checkExpiredTasks commit error: %v", err)
			continue
		}

		log.Printf("checkExpiredTasks: task %d (%s) ended, unfroze %.2f to business %d", t.taskID, t.title, unfrozenAmount, t.businessID)
	}
}

// resetDailyClaimCount 重置每日认领数
func resetDailyClaimCount(db *sql.DB) {
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	tomorrowStart := todayStart.AddDate(0, 0, 1)

	// 查找 daily_claim_reset < 今天0点 的用户
	rows, err := db.Query(`SELECT id FROM users WHERE daily_claim_reset IS NOT NULL AND daily_claim_reset < ?`, todayStart)
	if err != nil {
		log.Printf("resetDailyClaimCount query error: %v", err)
		return
	}
	defer rows.Close()

	var userIDs []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			log.Printf("resetDailyClaimCount scan error: %v", err)
			continue
		}
		userIDs = append(userIDs, id)
	}

	if len(userIDs) == 0 {
		return
	}

	// 重置 daily_claim_count = 0, daily_claim_reset = 明天0点
	for _, userID := range userIDs {
		_, err := db.Exec(`UPDATE users SET daily_claim_count = 0, daily_claim_reset = ? WHERE id = ?`, tomorrowStart, userID)
		if err != nil {
			log.Printf("resetDailyClaimCount update error for user %d: %v", userID, err)
			continue
		}
		log.Printf("resetDailyClaimCount: reset for user %d", userID)
	}
}
