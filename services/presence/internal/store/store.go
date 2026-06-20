package store

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const (
	CodeInvalidConnection = "INVALID_CONNECTION"
	CodeStaleConnection   = "STALE_CONNECTION"
)

var (
	ErrInvalidConnection = errors.New("invalid connection")
	ErrStaleConnection   = errors.New("stale connection")
)

// ConnectionMeta is Presence-owned online route metadata.
// ConnectionMeta 是 Presence 持有的在线路由元数据。
type ConnectionMeta struct {
	SessionID        string
	ConnID           string
	EdgeID           string
	UserID           string
	DeviceID         string
	GatewayID        string
	LastClientSeq    int64
	LastDeliveredSeq int64
}

// Store owns Presence Redis keys and their consistency rules.
// Store 管理 Presence Redis key 及一致性规则。
type Store struct {
	client     *goredis.Client
	defaultTTL time.Duration
}

func New(client *goredis.Client, defaultTTL time.Duration) *Store {
	if defaultTTL <= 0 {
		defaultTTL = 90 * time.Second
	}
	return &Store{
		client:     client,
		defaultTTL: defaultTTL,
	}
}

func (s *Store) Upsert(ctx context.Context, conn ConnectionMeta, ttl time.Duration) error {
	if err := validateConnection(conn); err != nil {
		return err
	}
	ttl = s.ttl(ttl)
	now := time.Now().UTC().Format(time.RFC3339)
	routeValue := encodeRouteValue(conn)

	pipe := s.client.TxPipeline()
	pipe.HSet(ctx, userKey(conn.UserID), conn.DeviceID, routeValue)
	pipe.SAdd(ctx, edgeKey(conn.EdgeID), conn.ConnID)
	pipe.HSet(ctx, metaKey(conn.ConnID), metaFields(conn, now))
	pipe.Expire(ctx, metaKey(conn.ConnID), ttl)
	pipe.HSet(ctx, routeKey(conn.SessionID), routeFields(conn, now))
	pipe.Expire(ctx, routeKey(conn.SessionID), ttl)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("upsert presence connection: %w", err)
	}
	return nil
}

func (s *Store) Refresh(ctx context.Context, sessionID, connID, edgeID string, ttl time.Duration) error {
	if sessionID == "" || connID == "" || edgeID == "" {
		return ErrInvalidConnection
	}
	route, err := s.client.HGetAll(ctx, routeKey(sessionID)).Result()
	if err != nil {
		return fmt.Errorf("read presence route: %w", err)
	}
	if !routeMatches(route, connID, edgeID) {
		return ErrStaleConnection
	}

	ttl = s.ttl(ttl)
	now := time.Now().UTC().Format(time.RFC3339)
	pipe := s.client.TxPipeline()
	pipe.HSet(ctx, metaKey(connID), "last_seen_at", now)
	pipe.Expire(ctx, metaKey(connID), ttl)
	pipe.Expire(ctx, routeKey(sessionID), ttl)
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("refresh presence connection: %w", err)
	}
	return nil
}

func (s *Store) RebindGateway(ctx context.Context, sessionID, connID, edgeID, gatewayID string, ttl time.Duration) error {
	if sessionID == "" || connID == "" || edgeID == "" || gatewayID == "" {
		return ErrInvalidConnection
	}
	route, err := s.client.HGetAll(ctx, routeKey(sessionID)).Result()
	if err != nil {
		return fmt.Errorf("read presence route: %w", err)
	}
	if !routeMatches(route, connID, edgeID) {
		return ErrStaleConnection
	}

	ttl = s.ttl(ttl)
	now := time.Now().UTC().Format(time.RFC3339)
	pipe := s.client.TxPipeline()
	pipe.HSet(ctx, routeKey(sessionID), "gateway_id", gatewayID, "last_seen_at", now)
	pipe.Expire(ctx, routeKey(sessionID), ttl)
	pipe.HSet(ctx, metaKey(connID), "gateway_id", gatewayID, "last_seen_at", now)
	pipe.Expire(ctx, metaKey(connID), ttl)
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("rebind presence gateway: %w", err)
	}
	return nil
}

func (s *Store) Remove(ctx context.Context, conn ConnectionMeta) (bool, error) {
	if err := validateConnection(conn); err != nil {
		return false, err
	}
	current, err := s.client.HGet(ctx, userKey(conn.UserID), conn.DeviceID).Result()
	if errors.Is(err, goredis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read presence user index: %w", err)
	}
	if current != encodeRouteValue(conn) {
		return false, nil
	}

	pipe := s.client.TxPipeline()
	pipe.HDel(ctx, userKey(conn.UserID), conn.DeviceID)
	pipe.SRem(ctx, edgeKey(conn.EdgeID), conn.ConnID)
	pipe.Del(ctx, metaKey(conn.ConnID), routeKey(conn.SessionID))
	_, err = pipe.Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("remove presence connection: %w", err)
	}
	return true, nil
}

func (s *Store) GetLiveRoutes(ctx context.Context, userID string) ([]ConnectionMeta, error) {
	if userID == "" {
		return nil, ErrInvalidConnection
	}
	index, err := s.client.HGetAll(ctx, userKey(userID)).Result()
	if err != nil {
		return nil, fmt.Errorf("read presence user index: %w", err)
	}

	routes := make([]ConnectionMeta, 0, len(index))
	for deviceID, routeValue := range index {
		edgeID, connID, sessionID, ok := decodeRouteValue(routeValue)
		if !ok {
			_ = s.client.HDel(ctx, userKey(userID), deviceID).Err()
			continue
		}
		meta, err := s.client.HGetAll(ctx, metaKey(connID)).Result()
		if err != nil {
			return nil, fmt.Errorf("read presence meta: %w", err)
		}
		route, err := s.client.HGetAll(ctx, routeKey(sessionID)).Result()
		if err != nil {
			return nil, fmt.Errorf("read presence route: %w", err)
		}
		if !metaMatches(meta, userID, deviceID, edgeID, connID, sessionID) || !routeMatches(route, connID, edgeID) {
			_ = s.client.HDel(ctx, userKey(userID), deviceID).Err()
			continue
		}
		routes = append(routes, metaFromHash(meta))
	}
	return routes, nil
}

func (s *Store) CleanupEdge(ctx context.Context, edgeID string, batchSize int) (int, error) {
	if edgeID == "" {
		return 0, ErrInvalidConnection
	}
	if batchSize <= 0 {
		batchSize = 512
	}
	connIDs, err := s.client.SMembers(ctx, edgeKey(edgeID)).Result()
	if err != nil {
		return 0, fmt.Errorf("read presence edge index: %w", err)
	}
	removed := 0
	for i, connID := range connIDs {
		if i >= batchSize {
			break
		}
		meta, err := s.client.HGetAll(ctx, metaKey(connID)).Result()
		if err != nil {
			return removed, fmt.Errorf("read presence meta: %w", err)
		}
		conn := metaFromHash(meta)
		if conn.ConnID == "" || conn.EdgeID != edgeID {
			if err := s.client.SRem(ctx, edgeKey(edgeID), connID).Err(); err != nil {
				return removed, err
			}
			removed++
			continue
		}
		route, err := s.client.HGetAll(ctx, routeKey(conn.SessionID)).Result()
		if err != nil {
			return removed, err
		}
		if !routeMatches(route, conn.ConnID, conn.EdgeID) {
			if ok, err := s.Remove(ctx, conn); err != nil {
				return removed, err
			} else if ok {
				removed++
			}
		}
	}
	return removed, nil
}

func (s *Store) ttl(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return s.defaultTTL
	}
	return ttl
}

func validateConnection(conn ConnectionMeta) error {
	if conn.SessionID == "" || conn.ConnID == "" || conn.EdgeID == "" || conn.UserID == "" || conn.DeviceID == "" {
		return ErrInvalidConnection
	}
	return nil
}

func routeMatches(route map[string]string, connID, edgeID string) bool {
	return route["conn_id"] == connID && route["edge_id"] == edgeID
}

func metaMatches(meta map[string]string, userID, deviceID, edgeID, connID, sessionID string) bool {
	return meta["user_id"] == userID &&
		meta["device_id"] == deviceID &&
		meta["edge_id"] == edgeID &&
		meta["conn_id"] == connID &&
		meta["session_id"] == sessionID
}

func metaFields(conn ConnectionMeta, now string) map[string]any {
	return map[string]any{
		"session_id":         conn.SessionID,
		"conn_id":            conn.ConnID,
		"edge_id":            conn.EdgeID,
		"user_id":            conn.UserID,
		"device_id":          conn.DeviceID,
		"gateway_id":         conn.GatewayID,
		"last_client_seq":    conn.LastClientSeq,
		"last_delivered_seq": conn.LastDeliveredSeq,
		"connected_at":       now,
		"last_seen_at":       now,
	}
}

func routeFields(conn ConnectionMeta, now string) map[string]any {
	return map[string]any{
		"session_id":         conn.SessionID,
		"conn_id":            conn.ConnID,
		"edge_id":            conn.EdgeID,
		"user_id":            conn.UserID,
		"device_id":          conn.DeviceID,
		"gateway_id":         conn.GatewayID,
		"last_client_seq":    conn.LastClientSeq,
		"last_delivered_seq": conn.LastDeliveredSeq,
		"last_seen_at":       now,
	}
}

func metaFromHash(meta map[string]string) ConnectionMeta {
	lastClientSeq, _ := strconv.ParseInt(meta["last_client_seq"], 10, 64)
	lastDeliveredSeq, _ := strconv.ParseInt(meta["last_delivered_seq"], 10, 64)
	return ConnectionMeta{
		SessionID:        meta["session_id"],
		ConnID:           meta["conn_id"],
		EdgeID:           meta["edge_id"],
		UserID:           meta["user_id"],
		DeviceID:         meta["device_id"],
		GatewayID:        meta["gateway_id"],
		LastClientSeq:    lastClientSeq,
		LastDeliveredSeq: lastDeliveredSeq,
	}
}

func encodeRouteValue(conn ConnectionMeta) string {
	return conn.EdgeID + ":" + conn.ConnID + ":" + conn.SessionID
}

func decodeRouteValue(value string) (edgeID, connID, sessionID string, ok bool) {
	parts := strings.Split(value, ":")
	if len(parts) != 3 {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], parts[0] != "" && parts[1] != "" && parts[2] != ""
}

func userKey(userID string) string {
	return "conn:user:" + userID
}

func edgeKey(edgeID string) string {
	return "conn:edge:" + edgeID
}

func metaKey(connID string) string {
	return "conn:meta:" + connID
}

func routeKey(sessionID string) string {
	return "session:route:" + sessionID
}

func CodeForError(err error) string {
	switch {
	case errors.Is(err, ErrInvalidConnection):
		return CodeInvalidConnection
	case errors.Is(err, ErrStaleConnection):
		return CodeStaleConnection
	default:
		return "INTERNAL_ERROR"
	}
}
