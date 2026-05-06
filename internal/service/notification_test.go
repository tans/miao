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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeSubmissionName(tt.raw, tt.fallback); got != tt.want {
				t.Fatalf("normalizeSubmissionName() = %q, want %q", got, tt.want)
			}
		})
	}
}
