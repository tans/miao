package service

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/tans/miao/internal/config"
	"github.com/tans/miao/internal/model"
	"github.com/tans/miao/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserExists      = errors.New("用户名已存在")
	ErrUserNotFound    = errors.New("用户不存在")
	ErrInvalidPassword = errors.New("密码错误")
	ErrInvalidUsername = errors.New("用户名或密码错误")
)

type AuthService struct {
	userRepo *repository.UserRepository
	cfg      *config.Config
}

func NewAuthService(userRepo *repository.UserRepository, cfg *config.Config) *AuthService {
	return &AuthService{
		userRepo: userRepo,
		cfg:      cfg,
	}
}

// Register creates a new user account
func (s *AuthService) Register(username, password, phone, role, realName, companyName string) (*model.User, error) {
	// Check if username already exists
	exists, err := s.userRepo.ExistsByUsername(username)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrUserExists
	}

	// Check if phone already exists
	if phone != "" {
		exists, err := s.userRepo.ExistsByPhone(phone)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, errors.New("手机号已被注册")
		}
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	// Create user
	user := &model.User{
		Username:     username,
		PasswordHash: string(hashedPassword),
		Phone:        phone,
		Role:         role,
		Status:       1, // 正常
		Balance:      0,
		FrozenAmount: 0,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Initialize creator-specific fields
	if role == "creator" || role == "creator,business" {
		user.Level = model.LevelBronze // 新注册创作者为青铜等级
		user.BehaviorScore = 100      // 初始行为分100
		user.TradeScore = 0           // 初始交易分0
		user.TotalScore = 100         // 总积分100
		user.MarginFrozen = 0
		user.DailyClaimCount = 0
		user.DailyClaimReset = time.Now().AddDate(0, 0, 1) // 次日重置
	}

	// Initialize business-specific fields
	if role == "business" || role == "creator,business" {
		user.BusinessVerified = false
		user.PublishCount = 0
	}

	err = s.userRepo.CreateUser(user)
	if err != nil {
		return nil, err
	}

	// Don't return the password hash
	user.PasswordHash = ""
	return user, nil
}

// Login authenticates a user and returns a JWT token
func (s *AuthService) Login(username, password string) (string, *model.User, error) {
	user, err := s.userRepo.GetUserByUsername(username)
	if err != nil {
		return "", nil, err
	}
	if user == nil {
		return "", nil, ErrInvalidUsername
	}

	// Verify password
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return "", nil, ErrInvalidPassword
	}

	// Check if user is active
	if user.Status != 1 {
		return "", nil, errors.New("账户已被禁用")
	}

	// Generate JWT token
	token, err := s.generateToken(user)
	if err != nil {
		return "", nil, err
	}

	// Clear password before returning
	user.PasswordHash = ""
	return token, user, nil
}

// generateToken creates a JWT token for the user
func (s *AuthService) generateToken(user *model.User) (string, error) {
	claims := jwt.MapClaims{
		"user_id":  user.ID,
		"username": user.Username,
		"role":     user.Role,
		"exp":      time.Now().Add(s.cfg.JWT.ExpireTime).Unix(),
		"iat":      time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.cfg.JWT.Secret))
}

// GetUserByID retrieves a user by ID
func (s *AuthService) GetUserByID(id int64) (*model.User, error) {
	user, err := s.userRepo.GetUserByID(id)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}
	user.PasswordHash = ""
	return user, nil
}

// UpdateProfile updates the user's profile
func (s *AuthService) UpdateProfile(userID int64, nickname, avatar string) (*model.User, error) {
	user, err := s.userRepo.GetUserByID(userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}

	if nickname != "" {
		user.Nickname = nickname
	}
	if avatar != "" {
		user.Avatar = avatar
	}

	err = s.userRepo.UpdateUser(user)
	if err != nil {
		return nil, err
	}

	user.PasswordHash = ""
	return user, nil
}

// UpdateUserForClaim 更新用户认领相关信息 (用于认领时)
func (s *AuthService) UpdateUserForClaim(userID int64, marginFrozen float64, dailyClaimCount int) error {
	return s.userRepo.UpdateUserForClaim(userID, marginFrozen, dailyClaimCount)
}

// ResetDailyClaimCount 重置每日认领数
func (s *AuthService) ResetDailyClaimCount(userID int64) error {
	return s.userRepo.ResetDailyClaimCount(userID)
}
