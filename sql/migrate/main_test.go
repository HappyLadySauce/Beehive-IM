package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestUserSchemaMigrationDefinesUsersTableAndUpdatedAtTrigger checks the users domain bootstrap SQL.
// TestUserSchemaMigrationDefinesUsersTableAndUpdatedAtTrigger 校验 users 域引导 SQL 中的用户表与 updated_at 触发器。
func TestUserSchemaMigrationDefinesUsersTableAndUpdatedAtTrigger(t *testing.T) {
	t.Helper()

	root := repoRootFromWorkingDir(t)
	userSQL := readRepoFile(t, root, filepath.Join("sql", "migrations", "users", "001_user.sql"))

	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS users",
		"username        VARCHAR(50) NOT NULL",
		"password_hash   VARCHAR(255)",
		"deleted_at      TIMESTAMPTZ",
		"CREATE OR REPLACE FUNCTION update_updated_at_column()",
		"CREATE TRIGGER trigger_users_updated_at",
		"CREATE TABLE IF NOT EXISTS user_profiles",
		"PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE",
	} {
		if !strings.Contains(userSQL, want) {
			t.Fatalf("user migration missing %q", want)
		}
	}
}

// TestAuthSchemaMigrationDefinesOAuthAndRefreshTokenTables checks auth persistence SQL.
// TestAuthSchemaMigrationDefinesOAuthAndRefreshTokenTables 校验 auth 持久化 SQL。
func TestAuthSchemaMigrationDefinesOAuthAndRefreshTokenTables(t *testing.T) {
	t.Helper()

	root := repoRootFromWorkingDir(t)
	authSQL := readRepoFile(t, root, filepath.Join("sql", "migrations", "auth", "002_auth.sql"))

	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS user_oauth_identities",
		"user_id          BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE",
		"CREATE UNIQUE INDEX IF NOT EXISTS uk_user_oauth_identities_provider_user_id",
		"CREATE UNIQUE INDEX IF NOT EXISTS uk_user_oauth_identities_user_provider",
		"CREATE TABLE IF NOT EXISTS refresh_tokens",
		"token_hash  CHAR(64) NOT NULL",
		"CREATE UNIQUE INDEX IF NOT EXISTS uk_refresh_tokens_token_hash",
		"CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_active",
	} {
		if !strings.Contains(authSQL, want) {
			t.Fatalf("auth migration missing %q", want)
		}
	}
}

// TestAuthSchemaMigrationReusesUserUpdatedAtTrigger wires auth tables to the shared trigger function.
// TestAuthSchemaMigrationReusesUserUpdatedAtTrigger 确认 auth 表复用 users 迁移中的 updated_at 触发器函数。
func TestAuthSchemaMigrationReusesUserUpdatedAtTrigger(t *testing.T) {
	t.Helper()

	root := repoRootFromWorkingDir(t)
	authSQL := readRepoFile(t, root, filepath.Join("sql", "migrations", "auth", "002_auth.sql"))

	for _, want := range []string{
		"EXECUTE FUNCTION update_updated_at_column()",
		"CREATE TRIGGER trigger_user_oauth_identities_updated_at",
	} {
		if !strings.Contains(authSQL, want) {
			t.Fatalf("auth migration missing %q", want)
		}
	}
}

// TestConversationSchemaMigrationDefinesConversationTables checks conversation persistence SQL.
// TestConversationSchemaMigrationDefinesConversationTables 校验 conversation 持久化 SQL。
func TestConversationSchemaMigrationDefinesConversationTables(t *testing.T) {
	t.Helper()

	root := repoRootFromWorkingDir(t)
	conversationSQL := readRepoFile(t, root, filepath.Join("sql", "migrations", "conversations", "003_conversation.sql"))

	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS conversations",
		"current_seq     BIGINT NOT NULL DEFAULT 0",
		"CONSTRAINT chk_conversations_status CHECK",
		"CREATE TABLE IF NOT EXISTS conversation_members",
		"PRIMARY KEY (conversation_id, user_id)",
		"CONSTRAINT chk_conversation_members_role CHECK",
		"CREATE TABLE IF NOT EXISTS conversation_settings",
		"trigger_conversations_updated_at",
	} {
		if !strings.Contains(conversationSQL, want) {
			t.Fatalf("conversation migration missing %q", want)
		}
	}
}

// TestMessageSchemaMigrationDefinesMessageReceiptAndOutboxTables checks message persistence SQL.
// TestMessageSchemaMigrationDefinesMessageReceiptAndOutboxTables 校验 message、receipt 和 outbox 持久化 SQL。
func TestMessageSchemaMigrationDefinesMessageReceiptAndOutboxTables(t *testing.T) {
	t.Helper()

	root := repoRootFromWorkingDir(t)
	messageSQL := readRepoFile(t, root, filepath.Join("sql", "migrations", "messages", "004_message.sql"))

	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS messages",
		"CREATE UNIQUE INDEX IF NOT EXISTS uk_messages_conversation_seq",
		"CREATE UNIQUE INDEX IF NOT EXISTS uk_messages_sender_device_client_msg",
		"CREATE TABLE IF NOT EXISTS message_receipts",
		"FOREIGN KEY (conversation_id, message_seq)",
		"CREATE TABLE IF NOT EXISTS outbox_events",
		"CONSTRAINT chk_outbox_events_status CHECK",
		"CREATE INDEX IF NOT EXISTS idx_outbox_events_pending",
	} {
		if !strings.Contains(messageSQL, want) {
			t.Fatalf("message migration missing %q", want)
		}
	}
}

// TestMigrationLayoutHasExpectedDomainFiles ensures the repo ships the current users/auth bootstrap pair.
// TestMigrationLayoutHasExpectedDomainFiles 确认仓库包含当前的 users/auth 引导迁移文件。
func TestMigrationLayoutHasExpectedDomainFiles(t *testing.T) {
	t.Helper()

	root := repoRootFromWorkingDir(t)
	for _, rel := range []string{
		filepath.Join("sql", "migrations", "users", "001_user.sql"),
		filepath.Join("sql", "migrations", "auth", "002_auth.sql"),
		filepath.Join("sql", "migrations", "conversations", "003_conversation.sql"),
		filepath.Join("sql", "migrations", "messages", "004_message.sql"),
	} {
		path := filepath.Join(root, rel)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected migration file %s: %v", rel, err)
		}
	}
}

func repoRootFromWorkingDir(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory failed: %v", err)
	}
	return filepath.Clean(filepath.Join(wd, "..", ".."))
}

func readRepoFile(t *testing.T, root, relativePath string) string {
	t.Helper()

	body, err := os.ReadFile(filepath.Join(root, relativePath))
	if err != nil {
		t.Fatalf("read %s failed: %v", relativePath, err)
	}
	return string(body)
}
