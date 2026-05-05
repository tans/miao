package model

import "testing"

func TestUserGetCommissionFallsBackForInvalidLevel(t *testing.T) {
	tests := []struct {
		name  string
		level UserLevel
	}{
		{name: "negative", level: UserLevel(-1)},
		{name: "too high", level: UserLevel(99)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := &User{Level: tt.level}

			if got := user.GetCommission(); got != 0.10 {
				t.Fatalf("GetCommission() = %v, want 0.10", got)
			}
		})
	}
}

func TestCalculateCreatorLevel(t *testing.T) {
	tests := []struct {
		adopted int
		want    UserLevel
	}{
		{0, LevelTrial},
		{2, LevelTrial},
		{3, LevelNewbie},
		{9, LevelNewbie},
		{10, LevelActive},
		{29, LevelActive},
		{30, LevelQuality},
		{79, LevelQuality},
		{80, LevelGold},
		{199, LevelGold},
		{200, LevelExclusive},
	}

	for _, tt := range tests {
		if got := CalculateCreatorLevel(tt.adopted); got != tt.want {
			t.Fatalf("CalculateCreatorLevel(%d) = %v, want %v", tt.adopted, got, tt.want)
		}
	}
}

func TestCreatorLevelSummaryFor(t *testing.T) {
	summary := CreatorLevelSummaryFor(9)
	if summary.Level != LevelNewbie {
		t.Fatalf("summary.Level = %v, want %v", summary.Level, LevelNewbie)
	}
	if summary.NextLevelName != "活跃创作者" {
		t.Fatalf("summary.NextLevelName = %q, want %q", summary.NextLevelName, "活跃创作者")
	}
	if summary.NeedCount != 1 {
		t.Fatalf("summary.NeedCount = %d, want 1", summary.NeedCount)
	}
	if len(summary.LevelRules) != 6 {
		t.Fatalf("summary.LevelRules len = %d, want 6", len(summary.LevelRules))
	}
}
