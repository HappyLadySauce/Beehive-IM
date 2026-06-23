package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	TypeDirect = "direct"
	TypeGroup  = "group"

	StatusActive = "active"
	StatusClosed = "closed"

	RoleOwner  = "owner"
	RoleAdmin  = "admin"
	RoleMember = "member"

	MemberStatusActive  = "active"
	MemberStatusRemoved = "removed"

	CodeInvalidArgument      = "INVALID_ARGUMENT"
	CodeConversationNotFound = "CONVERSATION_NOT_FOUND"
	CodeMemberNotFound       = "MEMBER_NOT_FOUND"
	CodePermissionDenied     = "PERMISSION_DENIED"
	CodeConversationClosed   = "CONVERSATION_CLOSED"
	CodeMemberMuted          = "MEMBER_MUTED"
	CodeInternal             = "INTERNAL_ERROR"
)

var (
	ErrInvalidArgument      = errors.New("invalid argument")
	ErrConversationNotFound = errors.New("conversation not found")
	ErrMemberNotFound       = errors.New("member not found")
	ErrPermissionDenied     = errors.New("permission denied")
	ErrConversationClosed   = errors.New("conversation is closed")
	ErrMemberMuted          = errors.New("member is muted")
)

// Conversation is the repository read model for a conversation.
// Conversation 是会话仓库读取模型。
type Conversation struct {
	ConversationID string
	Type           string
	Status         string
	Title          string
	OwnerUserID    string
	CurrentSeq     int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Member is the repository read model for a conversation member.
// Member 是会话成员仓库读取模型。
type Member struct {
	ConversationID   string
	UserID           string
	Role             string
	Status           string
	MutedUntil       *time.Time
	JoinedAt         time.Time
	UpdatedAt        time.Time
	VisibleFromSeq   int64
	VisibleToSeq     int64
	LastReadSeq      int64
	LastDeliveredSeq int64
	LastReadAt       *time.Time
}

// Settings is the repository read model for per-user conversation settings.
// Settings 是用户级会话设置仓库读取模型。
type Settings struct {
	ConversationID string
	UserID         string
	Pinned         bool
	MutedUntil     *time.Time
	Remark         string
	UpdatedAt      time.Time
}

// ListItem is one inbox row returned by Conversation service.
// ListItem 是 Conversation 服务返回的一条会话列表行。
type ListItem struct {
	Conversation Conversation
	Member       Member
	Settings     Settings
	MemberCount  int32
}

// ReadPermission carries the readable sequence range for a member.
// ReadPermission 携带成员可读消息序列范围。
type ReadPermission struct {
	VisibleFromSeq int64
	VisibleToSeq   int64
}

type MemberInput struct {
	UserID string
	Role   string
}

type CreateInput struct {
	CreatorUserID string
	Type          string
	Title         string
	Members       []MemberInput
}

type UpdateSettingsInput struct {
	ConversationID string
	ActorUserID    string
	TargetUserID   string
	Pinned         bool
	MutedUntil     string
	Remark         string
}

type querier interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

// Repository owns conversation persistence and permission checks.
// Repository 管理会话持久化与权限校验。
type Repository struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, input CreateInput) (Conversation, []Member, error) {
	if r == nil || r.db == nil {
		return Conversation{}, nil, errors.New("conversation repository is not initialized")
	}
	input.CreatorUserID = normalizeID(input.CreatorUserID)
	input.Type = normalizeConversationType(input.Type)
	input.Title = normalizeTitle(input.Title)
	if input.CreatorUserID == "" {
		return Conversation{}, nil, fmt.Errorf("%w: creator_user_id is required", ErrInvalidArgument)
	}
	if !isConversationType(input.Type) {
		return Conversation{}, nil, fmt.Errorf("%w: unsupported conversation type", ErrInvalidArgument)
	}

	members, err := normalizeMemberInputs(input.CreatorUserID, input.Members)
	if err != nil {
		return Conversation{}, nil, err
	}
	if input.Type == TypeDirect && len(members) != 2 {
		return Conversation{}, nil, fmt.Errorf("%w: direct conversation requires exactly two members", ErrInvalidArgument)
	}
	if input.Type == TypeGroup && len(members) < 3 {
		return Conversation{}, nil, fmt.Errorf("%w: group conversation requires at least three members", ErrInvalidArgument)
	}
	var directLow, directHigh string
	if input.Type == TypeDirect {
		directLow, directHigh = directPair(members[0].UserID, members[1].UserID)
		if directLow == "" || directHigh == "" {
			return Conversation{}, nil, fmt.Errorf("%w: direct conversation users must be different", ErrInvalidArgument)
		}
		if conversation, existingMembers, found, err := r.getDirectConversation(ctx, directLow, directHigh); err != nil {
			return Conversation{}, nil, err
		} else if found {
			return conversation, existingMembers, nil
		}
	}
	conversationID := uuid.NewString()

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Conversation{}, nil, fmt.Errorf("begin conversation create transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err = tx.Exec(ctx, `
INSERT INTO conversations (conversation_id, type, status, title, owner_user_id)
VALUES ($1, $2, 'active', $3, $4)`, conversationID, input.Type, input.Title, input.CreatorUserID); err != nil {
		return Conversation{}, nil, fmt.Errorf("insert conversation: %w", err)
	}

	for _, member := range members {
		if _, err = tx.Exec(ctx, `
INSERT INTO conversation_members (conversation_id, user_id, role, status, visible_from_seq, visible_to_seq, last_read_seq, last_delivered_seq)
VALUES ($1, $2, $3, 'active', 1, 0, 0, 0)`, conversationID, member.UserID, member.Role); err != nil {
			return Conversation{}, nil, fmt.Errorf("insert conversation member: %w", err)
		}
	}

	if input.Type == TypeDirect {
		var mappedID string
		err = tx.QueryRow(ctx, `
INSERT INTO direct_conversations (conversation_id, user_low, user_high)
VALUES ($1, $2, $3)
ON CONFLICT (user_low, user_high) DO NOTHING
RETURNING conversation_id`, conversationID, directLow, directHigh).Scan(&mappedID)
		if errors.Is(err, pgx.ErrNoRows) {
			_ = tx.Rollback(ctx)
			conversation, members, found, lookupErr := r.getDirectConversation(ctx, directLow, directHigh)
			if lookupErr != nil {
				return Conversation{}, nil, lookupErr
			}
			if !found {
				return Conversation{}, nil, errors.New("direct conversation conflict was not visible after insert")
			}
			return conversation, members, nil
		}
		if err != nil {
			return Conversation{}, nil, fmt.Errorf("insert direct conversation mapping: %w", err)
		}
	}

	conversation, err := queryConversation(ctx, tx, conversationID)
	if err != nil {
		return Conversation{}, nil, err
	}
	createdMembers, err := queryMembers(ctx, tx, conversationID)
	if err != nil {
		return Conversation{}, nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Conversation{}, nil, fmt.Errorf("commit conversation create transaction: %w", err)
	}
	return conversation, createdMembers, nil
}

func (r *Repository) Get(ctx context.Context, conversationID, requesterUserID string) (Conversation, []Member, Settings, error) {
	if r == nil || r.db == nil {
		return Conversation{}, nil, Settings{}, errors.New("conversation repository is not initialized")
	}
	conversationID = normalizeID(conversationID)
	requesterUserID = normalizeID(requesterUserID)
	if conversationID == "" || requesterUserID == "" {
		return Conversation{}, nil, Settings{}, fmt.Errorf("%w: conversation_id and requester_user_id are required", ErrInvalidArgument)
	}
	if err := requireActiveMember(ctx, r.db, conversationID, requesterUserID); err != nil {
		return Conversation{}, nil, Settings{}, err
	}
	conversation, err := queryConversation(ctx, r.db, conversationID)
	if err != nil {
		return Conversation{}, nil, Settings{}, err
	}
	members, err := queryMembers(ctx, r.db, conversationID)
	if err != nil {
		return Conversation{}, nil, Settings{}, err
	}
	settings, err := querySettings(ctx, r.db, conversationID, requesterUserID)
	if err != nil {
		return Conversation{}, nil, Settings{}, err
	}
	return conversation, members, settings, nil
}

func (r *Repository) List(ctx context.Context, userID string, limit, offset int32) ([]ListItem, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("conversation repository is not initialized")
	}
	userID = normalizeID(userID)
	if userID == "" {
		return nil, fmt.Errorf("%w: user_id is required", ErrInvalidArgument)
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := r.db.Query(ctx, `
SELECT c.conversation_id,
       c.type,
       c.status,
       c.title,
       c.owner_user_id,
       c.current_seq,
       c.created_at,
       c.updated_at,
       m.conversation_id,
       m.user_id,
       m.role,
       m.status,
       m.muted_until,
       m.joined_at,
       m.updated_at,
       m.visible_from_seq,
       m.visible_to_seq,
       m.last_read_seq,
       m.last_delivered_seq,
       m.last_read_at,
       COALESCE(s.conversation_id, c.conversation_id),
       COALESCE(s.user_id, m.user_id),
       COALESCE(s.pinned, false),
       s.muted_until,
       COALESCE(s.remark, ''),
       COALESCE(s.updated_at, NOW()),
       (
           SELECT COUNT(*)
           FROM conversation_members cm
           WHERE cm.conversation_id = c.conversation_id AND cm.status = 'active'
       ) AS member_count
FROM conversations c
JOIN conversation_members m ON m.conversation_id = c.conversation_id
LEFT JOIN conversation_settings s ON s.conversation_id = c.conversation_id AND s.user_id = m.user_id
WHERE m.user_id = $1
  AND m.status = 'active'
  AND c.status = 'active'
  AND c.deleted_at IS NULL
ORDER BY COALESCE(s.pinned, false) DESC, c.updated_at DESC
LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query conversations: %w", err)
	}
	defer rows.Close()

	var items []ListItem
	for rows.Next() {
		item, err := scanListItem(rows)
		if err != nil {
			return nil, fmt.Errorf("scan conversation list item: %w", err)
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("scan conversations: %w", err)
	}
	return items, nil
}

func (r *Repository) AddMembers(ctx context.Context, conversationID, actorUserID string, inputs []MemberInput) ([]Member, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("conversation repository is not initialized")
	}
	conversationID = normalizeID(conversationID)
	actorUserID = normalizeID(actorUserID)
	if conversationID == "" || actorUserID == "" {
		return nil, fmt.Errorf("%w: conversation_id and actor_user_id are required", ErrInvalidArgument)
	}
	members, err := normalizeMemberInputs("", inputs)
	if err != nil {
		return nil, err
	}
	if len(members) == 0 {
		return nil, fmt.Errorf("%w: members are required", ErrInvalidArgument)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin add members transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err = requireManager(ctx, tx, conversationID, actorUserID); err != nil {
		return nil, err
	}
	conversation, err := queryConversation(ctx, tx, conversationID)
	if err != nil {
		return nil, err
	}
	if conversation.Type == TypeDirect {
		return nil, fmt.Errorf("%w: direct conversation does not support adding members", ErrInvalidArgument)
	}
	visibleFromSeq := conversation.CurrentSeq + 1
	for _, member := range members {
		if member.Role == RoleOwner {
			return nil, fmt.Errorf("%w: owner role cannot be assigned by add members", ErrInvalidArgument)
		}
		if _, err = tx.Exec(ctx, `
INSERT INTO conversation_members (conversation_id, user_id, role, status, muted_until, visible_from_seq, visible_to_seq)
VALUES ($1, $2, $3, 'active', NULL, $4, 0)
ON CONFLICT (conversation_id, user_id)
DO UPDATE SET role = EXCLUDED.role,
              status = 'active',
              muted_until = NULL,
              visible_from_seq = EXCLUDED.visible_from_seq,
              visible_to_seq = 0`, conversationID, member.UserID, member.Role, visibleFromSeq); err != nil {
			return nil, fmt.Errorf("upsert conversation member: %w", err)
		}
	}
	next, err := queryMembers(ctx, tx, conversationID)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit add members transaction: %w", err)
	}
	return next, nil
}

func (r *Repository) RemoveMembers(ctx context.Context, conversationID, actorUserID string, userIDs []string) (int32, error) {
	if r == nil || r.db == nil {
		return 0, errors.New("conversation repository is not initialized")
	}
	conversationID = normalizeID(conversationID)
	actorUserID = normalizeID(actorUserID)
	targets := normalizeIDs(userIDs)
	if conversationID == "" || actorUserID == "" || len(targets) == 0 {
		return 0, fmt.Errorf("%w: conversation_id, actor_user_id and user_ids are required", ErrInvalidArgument)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin remove members transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err = requireManager(ctx, tx, conversationID, actorUserID); err != nil {
		return 0, err
	}
	conversation, err := queryConversation(ctx, tx, conversationID)
	if err != nil {
		return 0, err
	}
	if conversation.Type == TypeDirect {
		return 0, fmt.Errorf("%w: direct conversation does not support removing members", ErrInvalidArgument)
	}
	for _, target := range targets {
		role, status, err := queryMemberRoleStatus(ctx, tx, conversationID, target)
		if err != nil {
			return 0, err
		}
		if status != MemberStatusActive {
			continue
		}
		if role == RoleOwner {
			return 0, fmt.Errorf("%w: owner cannot be removed", ErrPermissionDenied)
		}
	}
	tag, err := tx.Exec(ctx, `
UPDATE conversation_members
SET status = 'removed',
    visible_to_seq = $3
WHERE conversation_id = $1 AND user_id = ANY($2) AND status = 'active'`, conversationID, targets, conversation.CurrentSeq)
	if err != nil {
		return 0, fmt.Errorf("remove conversation members: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit remove members transaction: %w", err)
	}
	return int32(tag.RowsAffected()), nil
}

func (r *Repository) LeaveConversation(ctx context.Context, conversationID, actorUserID string) error {
	if r == nil || r.db == nil {
		return errors.New("conversation repository is not initialized")
	}
	conversationID = normalizeID(conversationID)
	actorUserID = normalizeID(actorUserID)
	if conversationID == "" || actorUserID == "" {
		return fmt.Errorf("%w: conversation_id and actor_user_id are required", ErrInvalidArgument)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin leave conversation transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	conversation, err := queryConversation(ctx, tx, conversationID)
	if err != nil {
		return err
	}
	if conversation.Type == TypeDirect {
		return fmt.Errorf("%w: direct conversation does not support leave", ErrInvalidArgument)
	}
	role, status, err := queryMemberRoleStatus(ctx, tx, conversationID, actorUserID)
	if err != nil {
		return err
	}
	if status != MemberStatusActive {
		return ErrPermissionDenied
	}
	if role == RoleOwner {
		return fmt.Errorf("%w: owner must transfer or dismiss before leaving", ErrPermissionDenied)
	}
	if _, err = tx.Exec(ctx, `
UPDATE conversation_members
SET status = 'removed',
    visible_to_seq = $3
WHERE conversation_id = $1 AND user_id = $2 AND status = 'active'`, conversationID, actorUserID, conversation.CurrentSeq); err != nil {
		return fmt.Errorf("leave conversation: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit leave conversation transaction: %w", err)
	}
	return nil
}

func (r *Repository) DismissConversation(ctx context.Context, conversationID, actorUserID string) error {
	if r == nil || r.db == nil {
		return errors.New("conversation repository is not initialized")
	}
	conversationID = normalizeID(conversationID)
	actorUserID = normalizeID(actorUserID)
	if conversationID == "" || actorUserID == "" {
		return fmt.Errorf("%w: conversation_id and actor_user_id are required", ErrInvalidArgument)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin dismiss conversation transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	conversation, err := queryConversation(ctx, tx, conversationID)
	if err != nil {
		return err
	}
	if conversation.Type == TypeDirect {
		return fmt.Errorf("%w: direct conversation does not support dismiss", ErrInvalidArgument)
	}
	role, status, err := queryMemberRoleStatus(ctx, tx, conversationID, actorUserID)
	if err != nil {
		return err
	}
	if role != RoleOwner || status != MemberStatusActive {
		return ErrPermissionDenied
	}
	if _, err = tx.Exec(ctx, `
UPDATE conversations
SET status = 'closed'
WHERE conversation_id = $1 AND status = 'active'`, conversationID); err != nil {
		return fmt.Errorf("close conversation: %w", err)
	}
	if _, err = tx.Exec(ctx, `
UPDATE conversation_members
SET status = 'removed',
    visible_to_seq = CASE WHEN visible_to_seq > 0 THEN visible_to_seq ELSE $2 END
WHERE conversation_id = $1 AND status = 'active'`, conversationID, conversation.CurrentSeq); err != nil {
		return fmt.Errorf("remove dismissed conversation members: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit dismiss conversation transaction: %w", err)
	}
	return nil
}

func (r *Repository) TransferOwner(ctx context.Context, conversationID, actorUserID, targetUserID string) (Member, Member, error) {
	if r == nil || r.db == nil {
		return Member{}, Member{}, errors.New("conversation repository is not initialized")
	}
	conversationID = normalizeID(conversationID)
	actorUserID = normalizeID(actorUserID)
	targetUserID = normalizeID(targetUserID)
	if conversationID == "" || actorUserID == "" || targetUserID == "" {
		return Member{}, Member{}, fmt.Errorf("%w: conversation_id, actor_user_id and target_user_id are required", ErrInvalidArgument)
	}
	if actorUserID == targetUserID {
		return Member{}, Member{}, fmt.Errorf("%w: target_user_id must be different from owner", ErrInvalidArgument)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Member{}, Member{}, fmt.Errorf("begin transfer owner transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	conversation, err := queryConversation(ctx, tx, conversationID)
	if err != nil {
		return Member{}, Member{}, err
	}
	if conversation.Type == TypeDirect {
		return Member{}, Member{}, fmt.Errorf("%w: direct conversation does not support owner transfer", ErrInvalidArgument)
	}
	actorRole, actorStatus, err := queryMemberRoleStatus(ctx, tx, conversationID, actorUserID)
	if err != nil {
		return Member{}, Member{}, err
	}
	if actorRole != RoleOwner || actorStatus != MemberStatusActive {
		return Member{}, Member{}, ErrPermissionDenied
	}
	_, targetStatus, err := queryMemberRoleStatus(ctx, tx, conversationID, targetUserID)
	if err != nil {
		return Member{}, Member{}, err
	}
	if targetStatus != MemberStatusActive {
		return Member{}, Member{}, ErrMemberNotFound
	}
	if _, err = tx.Exec(ctx, `
UPDATE conversation_members
SET role = 'admin'
WHERE conversation_id = $1 AND user_id = $2`, conversationID, actorUserID); err != nil {
		return Member{}, Member{}, fmt.Errorf("downgrade old owner: %w", err)
	}
	if _, err = tx.Exec(ctx, `
UPDATE conversation_members
SET role = 'owner'
WHERE conversation_id = $1 AND user_id = $2`, conversationID, targetUserID); err != nil {
		return Member{}, Member{}, fmt.Errorf("promote new owner: %w", err)
	}
	if _, err = tx.Exec(ctx, `
UPDATE conversations
SET owner_user_id = $2
WHERE conversation_id = $1`, conversationID, targetUserID); err != nil {
		return Member{}, Member{}, fmt.Errorf("update conversation owner: %w", err)
	}
	oldOwner, err := queryMember(ctx, tx, conversationID, actorUserID)
	if err != nil {
		return Member{}, Member{}, err
	}
	newOwner, err := queryMember(ctx, tx, conversationID, targetUserID)
	if err != nil {
		return Member{}, Member{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Member{}, Member{}, fmt.Errorf("commit transfer owner transaction: %w", err)
	}
	return oldOwner, newOwner, nil
}

func (r *Repository) UpdateMemberRole(ctx context.Context, conversationID, actorUserID, targetUserID, role string) (Member, error) {
	if r == nil || r.db == nil {
		return Member{}, errors.New("conversation repository is not initialized")
	}
	conversationID = normalizeID(conversationID)
	actorUserID = normalizeID(actorUserID)
	targetUserID = normalizeID(targetUserID)
	role = normalizeRole(role, RoleMember)
	if conversationID == "" || actorUserID == "" || targetUserID == "" {
		return Member{}, fmt.Errorf("%w: conversation_id, actor_user_id and target_user_id are required", ErrInvalidArgument)
	}
	if role != RoleAdmin && role != RoleMember {
		return Member{}, fmt.Errorf("%w: role must be admin or member", ErrInvalidArgument)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Member{}, fmt.Errorf("begin update member role transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err = requireManager(ctx, tx, conversationID, actorUserID); err != nil {
		return Member{}, err
	}
	currentRole, status, err := queryMemberRoleStatus(ctx, tx, conversationID, targetUserID)
	if err != nil {
		return Member{}, err
	}
	if status != MemberStatusActive {
		return Member{}, ErrMemberNotFound
	}
	if currentRole == RoleOwner {
		return Member{}, fmt.Errorf("%w: owner role cannot be changed", ErrPermissionDenied)
	}
	if _, err = tx.Exec(ctx, `
UPDATE conversation_members
SET role = $3
WHERE conversation_id = $1 AND user_id = $2`, conversationID, targetUserID, role); err != nil {
		return Member{}, fmt.Errorf("update member role: %w", err)
	}
	member, err := queryMember(ctx, tx, conversationID, targetUserID)
	if err != nil {
		return Member{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Member{}, fmt.Errorf("commit update member role transaction: %w", err)
	}
	return member, nil
}

func (r *Repository) UpdateSettings(ctx context.Context, input UpdateSettingsInput) (Settings, error) {
	if r == nil || r.db == nil {
		return Settings{}, errors.New("conversation repository is not initialized")
	}
	input.ConversationID = normalizeID(input.ConversationID)
	input.ActorUserID = normalizeID(input.ActorUserID)
	input.TargetUserID = normalizeID(input.TargetUserID)
	if input.TargetUserID == "" {
		input.TargetUserID = input.ActorUserID
	}
	if input.ConversationID == "" || input.ActorUserID == "" || input.TargetUserID == "" {
		return Settings{}, fmt.Errorf("%w: conversation_id, actor_user_id and target_user_id are required", ErrInvalidArgument)
	}
	if input.ActorUserID != input.TargetUserID {
		return Settings{}, fmt.Errorf("%w: users can only update their own conversation settings", ErrPermissionDenied)
	}
	if err := requireActiveMember(ctx, r.db, input.ConversationID, input.ActorUserID); err != nil {
		return Settings{}, err
	}
	mutedUntil, err := parseOptionalTime(input.MutedUntil)
	if err != nil {
		return Settings{}, err
	}
	var mutedValue any
	if mutedUntil != nil {
		mutedValue = *mutedUntil
	}
	remark := normalizeRemark(input.Remark)

	if _, err = r.db.Exec(ctx, `
INSERT INTO conversation_settings (conversation_id, user_id, pinned, muted_until, remark)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (conversation_id, user_id)
DO UPDATE SET pinned = EXCLUDED.pinned,
              muted_until = EXCLUDED.muted_until,
              remark = EXCLUDED.remark`, input.ConversationID, input.TargetUserID, input.Pinned, mutedValue, remark); err != nil {
		return Settings{}, fmt.Errorf("upsert conversation settings: %w", err)
	}
	return querySettings(ctx, r.db, input.ConversationID, input.TargetUserID)
}

func (r *Repository) CheckSendPermission(ctx context.Context, conversationID, userID string) error {
	if r == nil || r.db == nil {
		return errors.New("conversation repository is not initialized")
	}
	return requireSendPermission(ctx, r.db, normalizeID(conversationID), normalizeID(userID))
}

func (r *Repository) CheckReadPermission(ctx context.Context, conversationID, userID string) (ReadPermission, error) {
	if r == nil || r.db == nil {
		return ReadPermission{}, errors.New("conversation repository is not initialized")
	}
	conversationID = normalizeID(conversationID)
	userID = normalizeID(userID)
	if conversationID == "" || userID == "" {
		return ReadPermission{}, fmt.Errorf("%w: conversation_id and user_id are required", ErrInvalidArgument)
	}
	var conversationStatus, memberStatus string
	var visibleFromSeq, visibleToSeq int64
	err := r.db.QueryRow(ctx, `
SELECT c.status, m.status, m.visible_from_seq, m.visible_to_seq
FROM conversations c
JOIN conversation_members m ON m.conversation_id = c.conversation_id
WHERE c.conversation_id = $1
  AND c.deleted_at IS NULL
  AND m.user_id = $2`, conversationID, userID).Scan(&conversationStatus, &memberStatus, &visibleFromSeq, &visibleToSeq)
	if errors.Is(err, pgx.ErrNoRows) {
		return ReadPermission{}, ErrPermissionDenied
	}
	if err != nil {
		return ReadPermission{}, fmt.Errorf("query read permission: %w", err)
	}
	if conversationStatus != StatusActive && memberStatus == MemberStatusActive {
		return ReadPermission{}, ErrConversationClosed
	}
	if memberStatus != MemberStatusActive && memberStatus != MemberStatusRemoved {
		return ReadPermission{}, ErrPermissionDenied
	}
	if visibleFromSeq <= 0 {
		visibleFromSeq = 1
	}
	return ReadPermission{
		VisibleFromSeq: visibleFromSeq,
		VisibleToSeq:   visibleToSeq,
	}, nil
}

func (r *Repository) MarkRead(ctx context.Context, conversationID, userID, cursorType string, seq int64) (Member, error) {
	if r == nil || r.db == nil {
		return Member{}, errors.New("conversation repository is not initialized")
	}
	conversationID = normalizeID(conversationID)
	userID = normalizeID(userID)
	cursorType = strings.ToLower(strings.TrimSpace(cursorType))
	if conversationID == "" || userID == "" || seq <= 0 {
		return Member{}, fmt.Errorf("%w: conversation_id, user_id and seq are required", ErrInvalidArgument)
	}
	if cursorType != "delivered" && cursorType != "read" {
		return Member{}, fmt.Errorf("%w: unsupported cursor_type", ErrInvalidArgument)
	}
	permission, err := r.CheckReadPermission(ctx, conversationID, userID)
	if err != nil {
		return Member{}, err
	}
	if seq < permission.VisibleFromSeq {
		return Member{}, fmt.Errorf("%w: seq is outside visible range", ErrInvalidArgument)
	}
	if permission.VisibleToSeq > 0 && seq > permission.VisibleToSeq {
		return Member{}, fmt.Errorf("%w: seq is outside visible range", ErrInvalidArgument)
	}
	if cursorType == "read" {
		_, err = r.db.Exec(ctx, `
UPDATE conversation_members
SET last_read_seq = GREATEST(last_read_seq, $3),
    last_delivered_seq = GREATEST(last_delivered_seq, $3),
    last_read_at = NOW()
WHERE conversation_id = $1 AND user_id = $2`, conversationID, userID, seq)
	} else {
		_, err = r.db.Exec(ctx, `
UPDATE conversation_members
SET last_delivered_seq = GREATEST(last_delivered_seq, $3)
WHERE conversation_id = $1 AND user_id = $2`, conversationID, userID, seq)
	}
	if err != nil {
		return Member{}, fmt.Errorf("mark conversation read state: %w", err)
	}
	return queryMember(ctx, r.db, conversationID, userID)
}

func (r *Repository) ResolveMessageRecipients(ctx context.Context, conversationID string) ([]string, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("conversation repository is not initialized")
	}
	conversationID = normalizeID(conversationID)
	if conversationID == "" {
		return nil, fmt.Errorf("%w: conversation_id is required", ErrInvalidArgument)
	}

	var status string
	if err := r.db.QueryRow(ctx, `
SELECT status
FROM conversations
WHERE conversation_id = $1 AND deleted_at IS NULL`, conversationID).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrConversationNotFound
		}
		return nil, fmt.Errorf("query conversation status: %w", err)
	}
	if status != StatusActive {
		return nil, ErrConversationClosed
	}

	rows, err := r.db.Query(ctx, `
SELECT user_id
FROM conversation_members
WHERE conversation_id = $1 AND status = 'active'
ORDER BY joined_at ASC, user_id ASC`, conversationID)
	if err != nil {
		return nil, fmt.Errorf("query message recipients: %w", err)
	}
	defer rows.Close()

	recipients := make([]string, 0, 8)
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, fmt.Errorf("scan message recipient: %w", err)
		}
		recipients = append(recipients, userID)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("scan message recipients: %w", err)
	}
	return recipients, nil
}

func (r *Repository) AllocateSeq(ctx context.Context, conversationID, userID string) (int64, error) {
	if r == nil || r.db == nil {
		return 0, errors.New("conversation repository is not initialized")
	}
	conversationID = normalizeID(conversationID)
	userID = normalizeID(userID)
	if conversationID == "" || userID == "" {
		return 0, fmt.Errorf("%w: conversation_id and user_id are required", ErrInvalidArgument)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin allocate sequence transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var status string
	err = tx.QueryRow(ctx, `
SELECT status
FROM conversations
WHERE conversation_id = $1 AND deleted_at IS NULL
FOR UPDATE`, conversationID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrConversationNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("lock conversation sequence: %w", err)
	}
	if status != StatusActive {
		return 0, ErrConversationClosed
	}
	if err = requireSendPermissionLockedConversation(ctx, tx, conversationID, userID); err != nil {
		return 0, err
	}

	var seq int64
	if err = tx.QueryRow(ctx, `
UPDATE conversations
SET current_seq = current_seq + 1
WHERE conversation_id = $1
RETURNING current_seq`, conversationID).Scan(&seq); err != nil {
		return 0, fmt.Errorf("allocate message sequence: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit allocate sequence transaction: %w", err)
	}
	return seq, nil
}

func queryConversation(ctx context.Context, q querier, conversationID string) (Conversation, error) {
	conversation, err := scanConversation(q.QueryRow(ctx, `
SELECT conversation_id, type, status, title, owner_user_id, current_seq, created_at, updated_at
FROM conversations
WHERE conversation_id = $1 AND deleted_at IS NULL`, conversationID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Conversation{}, ErrConversationNotFound
	}
	if err != nil {
		return Conversation{}, fmt.Errorf("query conversation: %w", err)
	}
	return conversation, nil
}

func (r *Repository) getDirectConversation(ctx context.Context, userLow, userHigh string) (Conversation, []Member, bool, error) {
	if r == nil || r.db == nil {
		return Conversation{}, nil, false, errors.New("conversation repository is not initialized")
	}
	var conversationID string
	err := r.db.QueryRow(ctx, `
SELECT conversation_id
FROM direct_conversations
WHERE user_low = $1 AND user_high = $2`, userLow, userHigh).Scan(&conversationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Conversation{}, nil, false, nil
	}
	if err != nil {
		return Conversation{}, nil, false, fmt.Errorf("query direct conversation mapping: %w", err)
	}
	conversation, err := queryConversation(ctx, r.db, conversationID)
	if err != nil {
		return Conversation{}, nil, false, err
	}
	members, err := queryMembers(ctx, r.db, conversationID)
	if err != nil {
		return Conversation{}, nil, false, err
	}
	return conversation, members, true, nil
}

func queryMembers(ctx context.Context, q querier, conversationID string) ([]Member, error) {
	rows, err := q.Query(ctx, `
SELECT conversation_id, user_id, role, status, muted_until, joined_at, updated_at,
       visible_from_seq, visible_to_seq, last_read_seq, last_delivered_seq, last_read_at
FROM conversation_members
WHERE conversation_id = $1
ORDER BY joined_at ASC, user_id ASC`, conversationID)
	if err != nil {
		return nil, fmt.Errorf("query conversation members: %w", err)
	}
	defer rows.Close()

	var members []Member
	for rows.Next() {
		member, err := scanMember(rows)
		if err != nil {
			return nil, err
		}
		members = append(members, member)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("scan conversation members: %w", err)
	}
	return members, nil
}

func queryMember(ctx context.Context, q querier, conversationID, userID string) (Member, error) {
	member, err := scanMember(q.QueryRow(ctx, `
SELECT conversation_id, user_id, role, status, muted_until, joined_at, updated_at,
       visible_from_seq, visible_to_seq, last_read_seq, last_delivered_seq, last_read_at
FROM conversation_members
WHERE conversation_id = $1 AND user_id = $2`, conversationID, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Member{}, ErrMemberNotFound
	}
	if err != nil {
		return Member{}, fmt.Errorf("query conversation member: %w", err)
	}
	return member, nil
}

func querySettings(ctx context.Context, q querier, conversationID, userID string) (Settings, error) {
	settings, err := scanSettings(q.QueryRow(ctx, `
SELECT conversation_id, user_id, pinned, muted_until, remark, updated_at
FROM conversation_settings
WHERE conversation_id = $1 AND user_id = $2`, conversationID, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Settings{
			ConversationID: conversationID,
			UserID:         userID,
			UpdatedAt:      time.Now().UTC(),
		}, nil
	}
	if err != nil {
		return Settings{}, fmt.Errorf("query conversation settings: %w", err)
	}
	return settings, nil
}

func queryMemberRoleStatus(ctx context.Context, q querier, conversationID, userID string) (string, string, error) {
	var role, status string
	err := q.QueryRow(ctx, `
SELECT role, status
FROM conversation_members
WHERE conversation_id = $1 AND user_id = $2`, conversationID, userID).Scan(&role, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", ErrMemberNotFound
	}
	if err != nil {
		return "", "", fmt.Errorf("query member role: %w", err)
	}
	return role, status, nil
}

func requireManager(ctx context.Context, q querier, conversationID, userID string) error {
	var conversationStatus, role, memberStatus string
	err := q.QueryRow(ctx, `
SELECT c.status, m.role, m.status
FROM conversations c
JOIN conversation_members m ON m.conversation_id = c.conversation_id
WHERE c.conversation_id = $1
  AND c.deleted_at IS NULL
  AND m.user_id = $2`, conversationID, userID).Scan(&conversationStatus, &role, &memberStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrPermissionDenied
	}
	if err != nil {
		return fmt.Errorf("query manager permission: %w", err)
	}
	if conversationStatus != StatusActive {
		return ErrConversationClosed
	}
	if memberStatus != MemberStatusActive {
		return ErrPermissionDenied
	}
	if role != RoleOwner && role != RoleAdmin {
		return ErrPermissionDenied
	}
	return nil
}

func requireActiveMember(ctx context.Context, q querier, conversationID, userID string) error {
	var conversationStatus, memberStatus string
	err := q.QueryRow(ctx, `
SELECT c.status, m.status
FROM conversations c
JOIN conversation_members m ON m.conversation_id = c.conversation_id
WHERE c.conversation_id = $1
  AND c.deleted_at IS NULL
  AND m.user_id = $2`, conversationID, userID).Scan(&conversationStatus, &memberStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrPermissionDenied
	}
	if err != nil {
		return fmt.Errorf("query active member permission: %w", err)
	}
	if conversationStatus != StatusActive {
		return ErrConversationClosed
	}
	if memberStatus != MemberStatusActive {
		return ErrPermissionDenied
	}
	return nil
}

func requireSendPermission(ctx context.Context, q querier, conversationID, userID string) error {
	if conversationID == "" || userID == "" {
		return fmt.Errorf("%w: conversation_id and user_id are required", ErrInvalidArgument)
	}
	var conversationStatus, memberStatus string
	var mutedUntil pgtype.Timestamptz
	err := q.QueryRow(ctx, `
SELECT c.status, m.status, m.muted_until
FROM conversations c
JOIN conversation_members m ON m.conversation_id = c.conversation_id
WHERE c.conversation_id = $1
  AND c.deleted_at IS NULL
  AND m.user_id = $2`, conversationID, userID).Scan(&conversationStatus, &memberStatus, &mutedUntil)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrPermissionDenied
	}
	if err != nil {
		return fmt.Errorf("query send permission: %w", err)
	}
	if conversationStatus != StatusActive {
		return ErrConversationClosed
	}
	if memberStatus != MemberStatusActive {
		return ErrPermissionDenied
	}
	if mutedUntil.Valid && mutedUntil.Time.After(time.Now().UTC()) {
		return ErrMemberMuted
	}
	return nil
}

func requireSendPermissionLockedConversation(ctx context.Context, q querier, conversationID, userID string) error {
	var memberStatus string
	var mutedUntil pgtype.Timestamptz
	err := q.QueryRow(ctx, `
SELECT status, muted_until
FROM conversation_members
WHERE conversation_id = $1 AND user_id = $2`, conversationID, userID).Scan(&memberStatus, &mutedUntil)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrPermissionDenied
	}
	if err != nil {
		return fmt.Errorf("query locked send permission: %w", err)
	}
	if memberStatus != MemberStatusActive {
		return ErrPermissionDenied
	}
	if mutedUntil.Valid && mutedUntil.Time.After(time.Now().UTC()) {
		return ErrMemberMuted
	}
	return nil
}

type rowScanner interface {
	Scan(...any) error
}

func scanConversation(scanner rowScanner) (Conversation, error) {
	var c Conversation
	if err := scanner.Scan(&c.ConversationID, &c.Type, &c.Status, &c.Title, &c.OwnerUserID, &c.CurrentSeq, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return Conversation{}, err
	}
	return c, nil
}

func scanMember(scanner rowScanner) (Member, error) {
	var m Member
	var muted pgtype.Timestamptz
	var lastReadAt pgtype.Timestamptz
	if err := scanner.Scan(
		&m.ConversationID,
		&m.UserID,
		&m.Role,
		&m.Status,
		&muted,
		&m.JoinedAt,
		&m.UpdatedAt,
		&m.VisibleFromSeq,
		&m.VisibleToSeq,
		&m.LastReadSeq,
		&m.LastDeliveredSeq,
		&lastReadAt,
	); err != nil {
		return Member{}, err
	}
	m.MutedUntil = timePtr(muted)
	m.LastReadAt = timePtr(lastReadAt)
	return m, nil
}

func scanListItem(scanner rowScanner) (ListItem, error) {
	var item ListItem
	var memberMuted, memberLastReadAt, settingsMuted pgtype.Timestamptz
	var memberCount int64
	err := scanner.Scan(
		&item.Conversation.ConversationID,
		&item.Conversation.Type,
		&item.Conversation.Status,
		&item.Conversation.Title,
		&item.Conversation.OwnerUserID,
		&item.Conversation.CurrentSeq,
		&item.Conversation.CreatedAt,
		&item.Conversation.UpdatedAt,
		&item.Member.ConversationID,
		&item.Member.UserID,
		&item.Member.Role,
		&item.Member.Status,
		&memberMuted,
		&item.Member.JoinedAt,
		&item.Member.UpdatedAt,
		&item.Member.VisibleFromSeq,
		&item.Member.VisibleToSeq,
		&item.Member.LastReadSeq,
		&item.Member.LastDeliveredSeq,
		&memberLastReadAt,
		&item.Settings.ConversationID,
		&item.Settings.UserID,
		&item.Settings.Pinned,
		&settingsMuted,
		&item.Settings.Remark,
		&item.Settings.UpdatedAt,
		&memberCount,
	)
	if err != nil {
		return ListItem{}, err
	}
	item.Member.MutedUntil = timePtr(memberMuted)
	item.Member.LastReadAt = timePtr(memberLastReadAt)
	item.Settings.MutedUntil = timePtr(settingsMuted)
	item.MemberCount = int32(memberCount)
	return item, nil
}

func scanSettings(scanner rowScanner) (Settings, error) {
	var s Settings
	var muted pgtype.Timestamptz
	if err := scanner.Scan(&s.ConversationID, &s.UserID, &s.Pinned, &muted, &s.Remark, &s.UpdatedAt); err != nil {
		return Settings{}, err
	}
	s.MutedUntil = timePtr(muted)
	return s, nil
}

func normalizeMemberInputs(ownerUserID string, inputs []MemberInput) ([]MemberInput, error) {
	byUser := make(map[string]MemberInput)
	if ownerUserID != "" {
		byUser[ownerUserID] = MemberInput{UserID: ownerUserID, Role: RoleOwner}
	}
	for _, input := range inputs {
		userID := normalizeID(input.UserID)
		if userID == "" {
			return nil, fmt.Errorf("%w: member user_id is required", ErrInvalidArgument)
		}
		role := normalizeRole(input.Role, RoleMember)
		if role == RoleOwner && ownerUserID != userID {
			return nil, fmt.Errorf("%w: owner role is reserved for conversation creator", ErrInvalidArgument)
		}
		if ownerUserID == userID {
			role = RoleOwner
		}
		if !isRole(role) {
			return nil, fmt.Errorf("%w: unsupported member role", ErrInvalidArgument)
		}
		byUser[userID] = MemberInput{UserID: userID, Role: role}
	}
	out := make([]MemberInput, 0, len(byUser))
	for _, member := range byUser {
		out = append(out, member)
	}
	return out, nil
}

func normalizeIDs(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		id := normalizeID(value)
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

func directPair(a, b string) (string, string) {
	a = normalizeID(a)
	b = normalizeID(b)
	if a == "" || b == "" || a == b {
		return "", ""
	}
	if a < b {
		return a, b
	}
	return b, a
}

func normalizeID(value string) string {
	return strings.TrimSpace(value)
}

func normalizeConversationType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return TypeGroup
	}
	return value
}

func normalizeRole(value, fallback string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return fallback
	}
	return value
}

func normalizeTitle(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 120 {
		return value[:120]
	}
	return value
}

func normalizeRemark(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 120 {
		return value[:120]
	}
	return value
}

func parseOptionalTime(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, fmt.Errorf("%w: muted_until must be RFC3339", ErrInvalidArgument)
	}
	utc := t.UTC()
	return &utc, nil
}

func isConversationType(value string) bool {
	return value == TypeDirect || value == TypeGroup
}

func isRole(value string) bool {
	return value == RoleOwner || value == RoleAdmin || value == RoleMember
}

func timePtr(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	t := value.Time.UTC()
	return &t
}

func CodeForError(err error) string {
	switch {
	case errors.Is(err, ErrInvalidArgument):
		return CodeInvalidArgument
	case errors.Is(err, ErrConversationNotFound):
		return CodeConversationNotFound
	case errors.Is(err, ErrMemberNotFound):
		return CodeMemberNotFound
	case errors.Is(err, ErrPermissionDenied):
		return CodePermissionDenied
	case errors.Is(err, ErrConversationClosed):
		return CodeConversationClosed
	case errors.Is(err, ErrMemberMuted):
		return CodeMemberMuted
	default:
		return CodeInternal
	}
}
