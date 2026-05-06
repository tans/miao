package service

import "testing"

func TestNormalizeSubmissionName(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		fallback string
		want     string
	}{
		{
			name:     "strip video prefix and extension",
			raw:      "视频稿件：样板间慢镜头.mp4",
			fallback: "样板间拍摄",
			want:     "样板间慢镜头",
		},
		{
			name:     "strip generic prefix and extension",
			raw:      "稿件: 样板间慢镜头.mov",
			fallback: "样板间拍摄",
			want:     "样板间慢镜头",
		},
		{
			name:     "fallback to task title",
			raw:      "",
			fallback: "样板间拍摄",
			want:     "样板间拍摄",
		},
		{
			name:     "generated tmp name falls back to task title",
			raw:      "视频稿件：tmp_6b51319536b61c03e5d6dc97de24cf41fd6fcdc5844226ee.mp4",
			fallback: "test1",
			want:     "test1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeSubmissionName(tt.raw, tt.fallback); got != tt.want {
				t.Fatalf("normalizeSubmissionName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeReviewComment(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "empty report prefix removed",
			raw:  "举报: ",
			want: "",
		},
		{
			name: "report prefix removed",
			raw:  "举报：涉嫌搬运",
			want: "涉嫌搬运",
		},
		{
			name: "plain reason preserved",
			raw:  "涉嫌搬运",
			want: "涉嫌搬运",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeReviewComment(tt.raw); got != tt.want {
				t.Fatalf("normalizeReviewComment() = %q, want %q", got, tt.want)
			}
		})
	}
}
