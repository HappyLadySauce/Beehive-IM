// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package logic

import (
	"context"
	"errors"
	"net/http"

	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/ticket"
	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateWsTicketLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateWsTicketLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateWsTicketLogic {
	return &CreateWsTicketLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateWsTicketLogic) CreateWsTicket(req *types.TicketRequest, r *http.Request) (resp *types.TicketResponse, err error) {
	userID, err := authenticatedUserID(l.svcCtx, r)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return nil, ticket.ErrMissingUserID
		}
		return nil, err
	}

	deviceID := req.DeviceId
	if deviceID == "" {
		deviceID = requestDeviceID(r)
	}

	t, err := l.svcCtx.Tickets.Issue(userID, deviceID, req.SessionId, r.Header.Get("Origin"))
	if err != nil {
		if errors.Is(err, ticket.ErrMissingUserID) {
			return nil, ticket.ErrMissingUserID
		}
		return nil, err
	}

	return &types.TicketResponse{
		Ticket:    t.Value,
		ExpiresIn: int64(l.svcCtx.Tickets.TTL().Seconds()),
		SessionId: t.SessionID,
		DeviceId:  t.DeviceID,
	}, nil
}
