package logic

import (
	"github.com/HappyLadySauce/Beehive-IM/services/auth/authservice"
	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/types"
)

func edgeAuthTokenResponse(in interface {
	GetAccessToken() string
	GetRefreshToken() string
	GetExpiresIn() int64
	GetTokenType() string
	GetUser() *authservice.UserInfo
}) *types.AuthTokenResponse {
	if in == nil {
		return &types.AuthTokenResponse{}
	}
	user := in.GetUser()
	resp := &types.AuthTokenResponse{
		AccessToken:  in.GetAccessToken(),
		RefreshToken: in.GetRefreshToken(),
		ExpiresIn:    in.GetExpiresIn(),
		TokenType:    in.GetTokenType(),
	}
	if user != nil {
		resp.User = types.AuthUserInfo{
			Id:       user.GetId(),
			Username: user.GetUsername(),
			Email:    user.GetEmail(),
			Avatar:   user.GetAvatar(),
		}
	}
	return resp
}
