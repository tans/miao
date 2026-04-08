package model

import "testing"

func TestNormalizeTaskCategoryToVideoOnly(t *testing.T) {
	testCases := []TaskCategory{
		CategoryCopywriting,
		CategoryDesign,
		CategoryVideo,
		CategoryPhotography,
		CategoryMusic,
		CategoryDev,
		CategoryOther,
		TaskCategory(0),
		TaskCategory(999),
	}

	for _, input := range testCases {
		if got := NormalizeTaskCategory(input); got != CategoryVideo {
			t.Fatalf("NormalizeTaskCategory(%d) = %d, want %d", input, got, CategoryVideo)
		}
	}
}
