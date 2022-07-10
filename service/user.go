package service

import (
	"context"
	"fmt"

	userDomain "github.com/oriser/bolt/user"
	"github.com/slack-go/slack"
)

func (h *Service) HandleAddUser(name string, user slack.User) error {
	if err := h.userStore.AddUser(context.Background(), &userDomain.User{
		FullName:           name,
		Email:              user.Profile.Email,
		Phone:              user.Profile.Phone,
		PaymentPreferences: nil,
		Timezone:           user.TZ,
		TransportID:        user.ID,
	}); err != nil {
		return fmt.Errorf("add user: %w", err)
	}
	return nil
}
