package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestUserSchemaMigrationDefinesUsersTableAndUpdatedAtTrigger checks the user domain bootstrap SQL.
// TestUserSchemaMigrationDefinesUsersTableAndUpdatedAtTrigger 校验 user 域引导 SQL 中的用户表与 updated_at 触发器。
func TestUserSchemaMigrationDefinesUsersTableAndUpdatedAtTrigger(t *testing.T) {
	t.Helper()

	root := repoRootFromWorkingDir(t)
	userSQL := readRepoFile(t, root, filepath.Join("sql", "migrations", "user", "000_user_users.sql"))

	for _, want := range []string{
		"CREATE SCHEMA IF NOT EXISTS user",
		"CREATE TABLE users",
		"username VARCHAR(50) NOT NULL UNIQUE",
		"password_hash VARCHAR(255) NOT NULL",
		"deleted_at TIMESTAMPTZ",
		"CREATE OR REPLACE FUNCTION update_updated_at_column()",
		"CREATE TRIGGER update_users_updated_at",
	} {
		if !strings.Contains(userSQL, want) {
			t.Fatalf("user migration missing %q", want)
		}
	}
}

// TestMessageSchemaMigrationDefinesConversationGraph checks IM core tables and referential integrity.
// TestMessageSchemaMigrationDefinesConversationGraph 校验 IM 核心表与外键约束。
func TestMessageSchemaMigrationDefinesConversationGraph(t *testing.T) {
	t.Helper()

	root := repoRootFromWorkingDir(t)
	messageSQL := readRepoFile(t, root, filepath.Join("sql", "migrations", "message", "000_message_core.sql"))

	for _, want := range []string{
		"CREATE TABLE conversations",
		"CREATE TABLE conversation_members",
		"CREATE TABLE messages",
		"CREATE TABLE message_deliveries",
		"REFERENCES conversations(id)",
		"REFERENCES users(id)",
		"message_id VARCHAR(64) NOT NULL UNIQUE",
		"CONSTRAINT messages_unique_client_message",
		"CONSTRAINT messages_unique_conversation_sequence",
		"CONSTRAINT messages_content_not_empty",
		"CONSTRAINT message_deliveries_unique_recipient",
	} {
		if !strings.Contains(messageSQL, want) {
			t.Fatalf("message migration missing %q", want)
		}
	}
}

// TestMessageSchemaMigrationReusesUserUpdatedAtTrigger wires message tables to the shared trigger function.
// TestMessageSchemaMigrationReusesUserUpdatedAtTrigger 确认 message 表复用 user 迁移中的 updated_at 触发器函数。
func TestMessageSchemaMigrationReusesUserUpdatedAtTrigger(t *testing.T) {
	t.Helper()

	root := repoRootFromWorkingDir(t)
	messageSQL := readRepoFile(t, root, filepath.Join("sql", "migrations", "message", "000_message_core.sql"))

	for _, want := range []string{
		"EXECUTE FUNCTION update_updated_at_column()",
		"CREATE TRIGGER update_conversations_updated_at",
		"CREATE TRIGGER update_messages_updated_at",
	} {
		if !strings.Contains(messageSQL, want) {
			t.Fatalf("message migration missing %q", want)
		}
	}
}

// TestMigrationLayoutHasExpectedDomainFiles ensures the repo ships the current user/message bootstrap pair.
// TestMigrationLayoutHasExpectedDomainFiles 确认仓库包含当前的 user/message 引导迁移文件。
func TestMigrationLayoutHasExpectedDomainFiles(t *testing.T) {
	t.Helper()

	root := repoRootFromWorkingDir(t)
	for _, rel := range []string{
		filepath.Join("sql", "migrations", "user", "000_user_users.sql"),
		filepath.Join("sql", "migrations", "message", "000_message_core.sql"),
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
