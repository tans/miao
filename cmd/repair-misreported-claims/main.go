package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/tans/miao/internal/config"
	"github.com/tans/miao/internal/database"
	"github.com/tans/miao/internal/model"
	"github.com/tans/miao/internal/repository"
	"github.com/tans/miao/internal/service"
)

const (
	fixComment = "系统修复：纠正前端审核结果码错误导致的误举报"
)

var bugIntroAt = time.Date(2026, 4, 26, 16, 19, 45, 0, time.FixedZone("CST", 8*3600))

type suspiciousClaim struct {
	ID            int64
	TaskID        int64
	CreatorID     int64
	Status        int
	ReviewResult  int
	ReviewComment string
	SubmitAt      time.Time
	ReviewAt      sql.NullTime
	UpdatedAt     time.Time
}

type taskSnapshot struct {
	ID           int64
	BusinessID   int64
	Title        string
	UnitPrice    float64
	AwardPrice   float64
	PaidAmount   float64
	Industries   string
	FrozenAmount float64
}

type userSnapshot struct {
	ID           int64
	Username     string
	Nickname     string
	Avatar       string
	Level        int
	Balance      float64
	AdoptedCount int
	ReportCount  int
}

type repairPlan struct {
	ClaimID                 int64
	TaskID                  int64
	TaskTitle               string
	CreatorID               int64
	BusinessID              int64
	ReviewAt                time.Time
	CreatorParticipation    float64
	CreatorAward            float64
	PlatformFee             float64
	MerchantBalanceRefunded float64
	PriorBusinessConsumeAbs float64
	PriorPlatformIncome     float64
	BusinessExpenseDelta    float64
	TaskPaidAmountDelta     float64
}

func main() {
	dbPath := flag.String("db", "/data/miao/data/miao.db", "sqlite database path")
	apply := flag.Bool("apply", false, "apply changes")
	publishClaimIDs := flag.String("publish-claim-ids", "", "comma-separated repaired claim ids to publish inspirations for")
	flag.Parse()

	db, err := sql.Open("sqlite3", *dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("ping db: %v", err)
	}

	if strings.TrimSpace(*publishClaimIDs) != "" {
		plans, err := loadPlansForPublishOnly(db, *publishClaimIDs)
		if err != nil {
			log.Fatalf("load publish-only plans: %v", err)
		}
		if len(plans) == 0 {
			log.Println("no claim ids to publish")
			return
		}
		if err := publishInspirations(*dbPath, plans); err != nil {
			log.Fatalf("publish inspirations: %v", err)
		}
		log.Printf("published inspirations for %d claims", len(plans))
		return
	}

	suspicious, skipped, err := findSuspiciousClaims(db)
	if err != nil {
		log.Fatalf("find suspicious claims: %v", err)
	}

	if len(skipped) > 0 {
		log.Printf("skipping %d older suspicious claims that predate the known frontend bug:", len(skipped))
		for _, item := range skipped {
			log.Printf("  claim=%d task=%d updated_at=%s", item.ID, item.TaskID, item.UpdatedAt.Format(time.RFC3339))
		}
	}

	if len(suspicious) == 0 {
		log.Println("no repairable claims found")
		return
	}

	plans, err := buildPlans(db, suspicious)
	if err != nil {
		log.Fatalf("build repair plans: %v", err)
	}

	log.Printf("repairable claims: %d", len(plans))
	var totalCreatorPayout float64
	var totalPlatformDelta float64
	var totalMerchantBalanceDeduct float64
	var totalTaskPaidDelta float64
	for _, plan := range plans {
		totalCreatorPayout += plan.CreatorParticipation + plan.CreatorAward
		totalPlatformDelta += plan.PlatformFee - plan.PriorPlatformIncome
		totalMerchantBalanceDeduct += plan.MerchantBalanceRefunded
		totalTaskPaidDelta += plan.TaskPaidAmountDelta
		log.Printf(
			"claim=%d task=%d(%s) creator=%d business=%d payout=%.2f+%.2f platform_delta=%.2f merchant_balance_deduct=%.2f task_paid_delta=%.2f",
			plan.ClaimID,
			plan.TaskID,
			plan.TaskTitle,
			plan.CreatorID,
			plan.BusinessID,
			plan.CreatorParticipation,
			plan.CreatorAward,
			plan.PlatformFee-plan.PriorPlatformIncome,
			plan.MerchantBalanceRefunded,
			plan.TaskPaidAmountDelta,
		)
	}
	log.Printf("totals: creator_payout=%.2f platform_delta=%.2f merchant_balance_deduct=%.2f task_paid_delta=%.2f",
		totalCreatorPayout, totalPlatformDelta, totalMerchantBalanceDeduct, totalTaskPaidDelta)

	if !*apply {
		log.Println("dry-run only; re-run with -apply to execute")
		return
	}

	now := time.Now()
	tx, err := db.Begin()
	if err != nil {
		log.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	creatorRepairCount := map[int64]int{}
	for _, plan := range plans {
		if err := applyClaimRepair(tx, plan, now); err != nil {
			log.Fatalf("apply claim %d: %v", plan.ClaimID, err)
		}
		creatorRepairCount[plan.CreatorID]++
	}

	for creatorID, count := range creatorRepairCount {
		var currentAdoptedCount int
		if err := tx.QueryRow(`SELECT adopted_count FROM users WHERE id = ?`, creatorID).Scan(&currentAdoptedCount); err != nil {
			log.Fatalf("load creator adopted count %d: %v", creatorID, err)
		}
		newAdoptedCount := currentAdoptedCount + count
		newLevel := model.CalculateCreatorLevel(newAdoptedCount)
		if _, err := tx.Exec(`
			UPDATE users
			SET adopted_count = ?,
				level = ?,
				report_count = (
					SELECT COUNT(*)
					FROM claims
					WHERE creator_id = ? AND review_result = ?
				),
				updated_at = ?
			WHERE id = ?
		`, newAdoptedCount, newLevel, creatorID, model.ReviewResultReport, now, creatorID); err != nil {
			log.Fatalf("update creator counters %d: %v", creatorID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		log.Fatalf("commit: %v", err)
	}

	log.Println("database transaction committed")

	if err := publishInspirations(*dbPath, plans); err != nil {
		log.Fatalf("publish inspirations: %v", err)
	}

	log.Println("repair completed")
}

func findSuspiciousClaims(db *sql.DB) ([]suspiciousClaim, []suspiciousClaim, error) {
	rows, err := db.Query(`
		SELECT id, task_id, creator_id, status, review_result,
			COALESCE(review_comment, ''),
			submit_at, review_at, updated_at
		FROM claims
		WHERE review_result = ?
			AND TRIM(COALESCE(review_comment, '')) = ''
			AND submit_at IS NOT NULL
		ORDER BY updated_at ASC, id ASC
	`, model.ReviewResultReport)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var suspicious []suspiciousClaim
	var skipped []suspiciousClaim
	for rows.Next() {
		var item suspiciousClaim
		if err := rows.Scan(
			&item.ID,
			&item.TaskID,
			&item.CreatorID,
			&item.Status,
			&item.ReviewResult,
			&item.ReviewComment,
			&item.SubmitAt,
			&item.ReviewAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, nil, err
		}
		if item.UpdatedAt.Before(bugIntroAt) {
			skipped = append(skipped, item)
			continue
		}
		suspicious = append(suspicious, item)
	}
	return suspicious, skipped, rows.Err()
}

func buildPlans(db *sql.DB, claims []suspiciousClaim) ([]repairPlan, error) {
	plans := make([]repairPlan, 0, len(claims))
	for _, claim := range claims {
		task, err := loadTask(db, claim.TaskID)
		if err != nil {
			return nil, fmt.Errorf("load task %d: %w", claim.TaskID, err)
		}
		creator, err := loadUser(db, claim.CreatorID)
		if err != nil {
			return nil, fmt.Errorf("load creator %d: %w", claim.CreatorID, err)
		}

		merchantBalanceRefunded, priorBusinessConsumeAbs, priorPlatformIncome, err := loadPriorAdjustments(db, claim.ID, task.BusinessID)
		if err != nil {
			return nil, fmt.Errorf("load prior adjustments claim %d: %w", claim.ID, err)
		}

		commissionRate := commissionRateForLevel(creator.Level)
		creatorParticipation := model.CreatorNetReward(task.UnitPrice, commissionRate)
		creatorAward := model.CreatorNetReward(task.AwardPrice, commissionRate)
		platformFee := model.PlatformCommissionAmount(task.UnitPrice+task.AwardPrice, commissionRate)
		reviewAt := claim.UpdatedAt
		if claim.ReviewAt.Valid {
			reviewAt = claim.ReviewAt.Time
		}

		priorConsumedBudget := priorBusinessConsumeAbs
		taskPaidDelta := round2((task.UnitPrice + task.AwardPrice) - priorConsumedBudget)
		if taskPaidDelta < 0 {
			taskPaidDelta = 0
		}

		plans = append(plans, repairPlan{
			ClaimID:                 claim.ID,
			TaskID:                  task.ID,
			TaskTitle:               task.Title,
			CreatorID:               creator.ID,
			BusinessID:              task.BusinessID,
			ReviewAt:                reviewAt,
			CreatorParticipation:    creatorParticipation,
			CreatorAward:            creatorAward,
			PlatformFee:             platformFee,
			MerchantBalanceRefunded: merchantBalanceRefunded,
			PriorBusinessConsumeAbs: priorBusinessConsumeAbs,
			PriorPlatformIncome:     priorPlatformIncome,
			BusinessExpenseDelta:    taskPaidDelta,
			TaskPaidAmountDelta:     taskPaidDelta,
		})
	}

	sort.Slice(plans, func(i, j int) bool { return plans[i].ClaimID < plans[j].ClaimID })
	return plans, nil
}

func applyClaimRepair(tx *sql.Tx, plan repairPlan, now time.Time) error {
	creator, err := loadUserTx(tx, plan.CreatorID)
	if err != nil {
		return err
	}
	businessUser, err := loadUserTx(tx, plan.BusinessID)
	if err != nil {
		return err
	}
	task, err := loadTaskTx(tx, plan.TaskID)
	if err != nil {
		return err
	}

	claimRewardTotal := round2(plan.CreatorParticipation + plan.CreatorAward)

	if _, err := tx.Exec(`
		UPDATE claims
		SET status = ?,
			review_at = ?,
			review_result = ?,
			review_comment = ?,
			creator_reward = ?,
			platform_fee = ?,
			updated_at = ?
		WHERE id = ?
	`, model.ClaimStatusApproved, plan.ReviewAt, model.ReviewResultPass, fixComment, claimRewardTotal, plan.PlatformFee, now, plan.ClaimID); err != nil {
		return fmt.Errorf("update claim: %w", err)
	}

	creatorBalance := creator.Balance
	if err := updateUserBalanceTx(tx, plan.CreatorID, round2(creatorBalance+plan.CreatorParticipation), now); err != nil {
		return fmt.Errorf("update creator participation balance: %w", err)
	}
	if err := insertTransaction(tx, plan.CreatorID, model.TransactionTypePayment, plan.CreatorParticipation, creatorBalance, round2(creatorBalance+plan.CreatorParticipation), "系统修复参与奖励: "+plan.TaskTitle, plan.ClaimID, now); err != nil {
		return fmt.Errorf("insert creator participation tx: %w", err)
	}

	creatorBalance = round2(creatorBalance + plan.CreatorParticipation)
	if err := updateUserBalanceTx(tx, plan.CreatorID, round2(creatorBalance+plan.CreatorAward), now); err != nil {
		return fmt.Errorf("update creator award balance: %w", err)
	}
	if err := insertTransaction(tx, plan.CreatorID, model.TransactionTypeAwardPayment, plan.CreatorAward, creatorBalance, round2(creatorBalance+plan.CreatorAward), "系统修复采纳奖励: "+plan.TaskTitle, plan.ClaimID, now); err != nil {
		return fmt.Errorf("insert creator award tx: %w", err)
	}

	if platformDelta := round2(plan.PlatformFee - plan.PriorPlatformIncome); platformDelta != 0 {
		if err := insertTransaction(tx, 0, model.TransactionTypePlatformIncome, platformDelta, 0, 0, "系统修复平台抽成调整: "+plan.TaskTitle, plan.ClaimID, now); err != nil {
			return fmt.Errorf("insert platform correction tx: %w", err)
		}
	}

	businessBalance := businessUser.Balance
	if plan.MerchantBalanceRefunded > 0 {
		nextBalance := round2(businessBalance - plan.MerchantBalanceRefunded)
		if err := updateUserBalanceTx(tx, plan.BusinessID, nextBalance, now); err != nil {
			return fmt.Errorf("update business balance: %w", err)
		}
		if err := insertTransaction(tx, plan.BusinessID, model.TransactionTypeConsume, -plan.MerchantBalanceRefunded, businessBalance, nextBalance, "系统修复扣回误退款: "+plan.TaskTitle, plan.ClaimID, now); err != nil {
			return fmt.Errorf("insert business refund reversal tx: %w", err)
		}
		businessBalance = nextBalance
	}

	if plan.BusinessExpenseDelta > 0 {
		if err := insertTransaction(tx, plan.BusinessID, model.TransactionTypeConsume, -plan.BusinessExpenseDelta, businessBalance, businessBalance, "系统修复补记采纳支出: "+plan.TaskTitle, plan.ClaimID, now); err != nil {
			return fmt.Errorf("insert business expense tx: %w", err)
		}
		task.PaidAmount = round2(task.PaidAmount + plan.TaskPaidAmountDelta)
		if _, err := tx.Exec(`UPDATE tasks SET paid_amount = ?, updated_at = ? WHERE id = ?`, task.PaidAmount, now, task.ID); err != nil {
			return fmt.Errorf("update task paid_amount: %w", err)
		}
	}

	return nil
}

func publishInspirations(dbPath string, plans []repairPlan) error {
	cfg := config.DatabaseConfig{Driver: string(database.DriverSQLite), Path: dbPath}
	db, err := database.InitDB(cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	creatorRepo := repository.NewCreatorRepository(db)
	claimInspirationService := service.NewClaimInspirationService(db)

	for _, plan := range plans {
		claim, err := creatorRepo.GetClaimByID(plan.ClaimID)
		if err != nil {
			return fmt.Errorf("load claim %d for inspiration publish: %w", plan.ClaimID, err)
		}
		task, err := loadTaskForInspiration(db, plan.TaskID)
		if err != nil {
			return fmt.Errorf("load task %d for inspiration publish: %w", plan.TaskID, err)
		}
		creator, err := creatorRepo.GetUserByID(plan.CreatorID)
		if err != nil {
			return fmt.Errorf("load creator %d for inspiration publish: %w", plan.CreatorID, err)
		}
		materials, err := creatorRepo.GetClaimMaterials(plan.ClaimID)
		if err != nil {
			return fmt.Errorf("load claim materials %d: %w", plan.ClaimID, err)
		}
		if _, err := claimInspirationService.PublishFromClaim(claim, task, creator, materials); err != nil {
			return fmt.Errorf("publish inspiration for claim %d: %w", plan.ClaimID, err)
		}
	}

	return nil
}

func loadPlansForPublishOnly(db *sql.DB, rawIDs string) ([]repairPlan, error) {
	parts := strings.Split(rawIDs, ",")
	plans := make([]repairPlan, 0, len(parts))
	seen := map[int64]struct{}{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		var claimID int64
		if _, err := fmt.Sscanf(part, "%d", &claimID); err != nil {
			return nil, fmt.Errorf("invalid claim id %q", part)
		}
		if _, ok := seen[claimID]; ok {
			continue
		}
		seen[claimID] = struct{}{}

		var plan repairPlan
		if err := db.QueryRow(`
			SELECT id, task_id, creator_id
			FROM claims
			WHERE id = ?
		`, claimID).Scan(&plan.ClaimID, &plan.TaskID, &plan.CreatorID); err != nil {
			return nil, fmt.Errorf("load claim %d: %w", claimID, err)
		}
		plans = append(plans, plan)
	}

	sort.Slice(plans, func(i, j int) bool { return plans[i].ClaimID < plans[j].ClaimID })
	return plans, nil
}

func loadTaskForInspiration(db database.DB, taskID int64) (*model.Task, error) {
	task := &model.Task{}
	err := db.QueryRow(`
		SELECT id, title, industries
		FROM tasks
		WHERE id = ?
	`, taskID).Scan(&task.ID, &task.Title, &task.Industries)
	if err != nil {
		return nil, err
	}
	return task, nil
}

func loadPriorAdjustments(db *sql.DB, claimID, businessID int64) (float64, float64, float64, error) {
	var merchantBalanceRefunded sql.NullFloat64
	if err := db.QueryRow(`
		SELECT COALESCE(SUM(balance_after - balance_before), 0)
		FROM transactions
		WHERE related_id = ? AND user_id = ?
	`, claimID, businessID).Scan(&merchantBalanceRefunded); err != nil {
		return 0, 0, 0, err
	}

	var priorBusinessConsumeAbs sql.NullFloat64
	if err := db.QueryRow(`
		SELECT COALESCE(SUM(CASE WHEN amount < 0 THEN -amount ELSE amount END), 0)
		FROM transactions
		WHERE related_id = ? AND user_id = ? AND type = ?
	`, claimID, businessID, model.TransactionTypeConsume).Scan(&priorBusinessConsumeAbs); err != nil {
		return 0, 0, 0, err
	}

	var priorPlatformIncome sql.NullFloat64
	if err := db.QueryRow(`
		SELECT COALESCE(SUM(amount), 0)
		FROM transactions
		WHERE related_id = ? AND user_id = 0 AND type = ?
	`, claimID, model.TransactionTypePlatformIncome).Scan(&priorPlatformIncome); err != nil {
		return 0, 0, 0, err
	}

	return round2(merchantBalanceRefunded.Float64), round2(priorBusinessConsumeAbs.Float64), round2(priorPlatformIncome.Float64), nil
}

func loadTask(db *sql.DB, taskID int64) (taskSnapshot, error) {
	return loadTaskRow(db.QueryRow(`
		SELECT id, business_id, title, unit_price, award_price, paid_amount, industries, frozen_amount
		FROM tasks WHERE id = ?
	`, taskID))
}

func loadTaskTx(tx *sql.Tx, taskID int64) (taskSnapshot, error) {
	return loadTaskRow(tx.QueryRow(`
		SELECT id, business_id, title, unit_price, award_price, paid_amount, industries, frozen_amount
		FROM tasks WHERE id = ?
	`, taskID))
}

func loadTaskRow(scanner rowScanner) (taskSnapshot, error) {
	var task taskSnapshot
	err := scanner.Scan(&task.ID, &task.BusinessID, &task.Title, &task.UnitPrice, &task.AwardPrice, &task.PaidAmount, &task.Industries, &task.FrozenAmount)
	return task, err
}

func loadUser(db *sql.DB, userID int64) (userSnapshot, error) {
	return loadUserRow(db.QueryRow(`
		SELECT id, username, nickname, avatar, level, balance, adopted_count, report_count
		FROM users WHERE id = ?
	`, userID))
}

func loadUserTx(tx *sql.Tx, userID int64) (userSnapshot, error) {
	return loadUserRow(tx.QueryRow(`
		SELECT id, username, nickname, avatar, level, balance, adopted_count, report_count
		FROM users WHERE id = ?
	`, userID))
}

func loadUserRow(scanner rowScanner) (userSnapshot, error) {
	var user userSnapshot
	err := scanner.Scan(&user.ID, &user.Username, &user.Nickname, &user.Avatar, &user.Level, &user.Balance, &user.AdoptedCount, &user.ReportCount)
	if err == nil {
		user.Level = int(model.CalculateCreatorLevel(user.AdoptedCount))
	}
	return user, err
}

func updateUserBalanceTx(tx *sql.Tx, userID int64, balance float64, now time.Time) error {
	_, err := tx.Exec(`UPDATE users SET balance = ?, updated_at = ? WHERE id = ?`, round2(balance), now, userID)
	return err
}

func insertTransaction(tx *sql.Tx, userID int64, txType model.TransactionType, amount, before, after float64, remark string, relatedID int64, createdAt time.Time) error {
	_, err := tx.Exec(`
		INSERT INTO transactions (user_id, type, amount, balance_before, balance_after, remark, related_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, userID, txType, round2(amount), round2(before), round2(after), strings.TrimSpace(remark), relatedID, createdAt)
	return err
}

func commissionRateForLevel(level int) float64 {
	switch level {
	case 4:
		return 0.05
	case 5:
		return 0.03
	default:
		return 0.10
	}
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}

type rowScanner interface {
	Scan(dest ...interface{}) error
}
