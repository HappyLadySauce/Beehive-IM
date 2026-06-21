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
	ConversationID string
	UserID         string
	Role           string
	Status         string
	MutedUntil     *time.Time
	JoinedAt       time.Time
	UpdatedAt      time.Time
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
INSERT INTO conversation_members (conversation_id, user_id, role, status)
VALUES ($1, $2, $3, 'active')`, conversationID, member.UserID, member.Role); err != nil {
			return Conversation{}, nil, fmt.Errorf("insert conversation member: %w", err)
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

func (r *Repository) List(ctx context.Context, userID string, limit, offset int32) ([]Conversation, error) {
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
SELECT c.conversation_id, c.type, c.status, c.title, c.owner_user_id, c.current_seq, c.created_at, c.updated_at
FROM conversations c
JOIN conversation_members m ON m.conversation_id = c.conversation_id
WHERE m.user_id = $1
  AND m.status = 'active'
  AND c.deleted_at IS NULL
ORDER BY c.updated_at DESC
LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query conversations: %w", err)
	}
	defer rows.Close()

	var conversations []Conversation
	for rows.Next() {
		conversation, err := scanConversation(rows)
		if err != nil {
			return nil, err
		}
		conversations = append(conversations, conversation)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("scan conversations: %w", err)
	}
	return conversations, nil
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
	for _, member := range members {
		if member.Role == RoleOwner {
			return nil, fmt.Errorf("%w: owner role cannot be assigned by add members", ErrInvalidArgument)
		}
		if _, err = tx.Exec(ctx, `
INSERT INTO conversation_members (conversation_id, user_id, role, status, muted_until)
VALUES ($1, $2, $3, 'active', NULL)
ON CONFLICT (conversation_id, user_id)
DO UPDATE SET role = EXCLUDED.role, status = 'active', muted_until = NULL`, conversationID, member.UserID, member.Role); err != nil {
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
SET status = 'removed'
WHERE conversation_id = $1 AND user_id = ANY($2) AND status = 'active'`, conversationID, targets)
	if err != nil {
		return 0, fmt.Errorf("remove conversation members: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit remove members transaction: %w", err)
	}
	return int32(tag.RowsAffected()), nil
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

func (r *Repository) CheckReadPermission(ctx context.Context, conversationID, userID string) error {
	if r == nil || r.db == nil {
		return errors.New("conversation repository is not initialized")
	}
	conversationID = normalizeID(conversationID)
	userID = normalizeID(userID)
	if conversationID == "" || userID == "" {
		return fmt.Errorf("%w: conversation_id and user_id are required", ErrInvalidArgument)
	}
	return requireActiveMember(ctx, r.db, conversationID, userID)
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

func queryMembers(ctx context.Context, q querier, conversationID string) ([]Member, error) {
	rows, err := q.Query(ctx, `
SELECT conversation_id, user_id, role, status, muted_until, joined_at, updated_at
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
SELECT conversation_id, user_id, role, status, muted_until, joined_at, updated_at
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
	if err := scanner.Scan(&m.ConversationID, &m.UserID, &m.Role, &m.Status, &muted, &m.JoinedAt, &m.UpdatedAt); err != nil {
		return Member{}, err
	}
	m.MutedUntil = timePtr(muted)
	return m, nil
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
