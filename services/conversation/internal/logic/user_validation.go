package logic

import (
	"context"
	"fmt"
	"strings"

	"github.com/HappyLadySauce/Beehive-IM/services/conversation/internal/repository"
	"github.com/HappyLadySauce/Beehive-IM/services/conversation/pb"
	"github.com/HappyLadySauce/Beehive-IM/services/user/userservice"
)

// validateUsersExist ensures all member ids refer to existing users.
// validateUsersExist 确保所有成员 ID 都对应已存在用户。
func validateUsersExist(ctx context.Context, client userservice.UserService, userIDs []string) error {
	ids := normalizeLogicIDs(userIDs)
	if len(ids) == 0 {
		return fmt.Errorf("%w: user ids are required", repository.ErrInvalidArgument)
	}
	resp, err := client.BatchGetUsers(ctx, &userservice.BatchGetUsersRequest{Ids: ids})
	if err != nil {
		return fmt.Errorf("batch get users rpc: %w", err)
	}
	if len(resp.GetMissingIds()) > 0 {
		return fmt.Errorf("%w: user not found: %v", repository.ErrInvalidArgument, resp.GetMissingIds())
	}
	if len(resp.GetUsers()) != len(ids) {
		return fmt.Errorf("%w: user not found", repository.ErrInvalidArgument)
	}
	return nil
}

func userIDsFromCreate(creator string, members []*pb.MemberInput) []string {
	ids := []string{creator}
	for _, member := range members {
		if member != nil {
			ids = append(ids, member.GetUserId())
		}
	}
	return normalizeLogicIDs(ids)
}

func userIDsFromMembers(members []*pb.MemberInput) []string {
	ids := make([]string, 0, len(members))
	for _, member := range members {
		if member != nil {
			ids = append(ids, member.GetUserId())
		}
	}
	return normalizeLogicIDs(ids)
}

func normalizeLogicIDs(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		id := strings.TrimSpace(value)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
