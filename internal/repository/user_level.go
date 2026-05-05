package repository

import "github.com/tans/miao/internal/model"

func normalizeCreatorUser(user *model.User) {
	if user == nil {
		return
	}
	user.RefreshEffectiveLevel()
}
