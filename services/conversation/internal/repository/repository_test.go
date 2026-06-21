package repository

import (
	"errors"
	"testing"
)

func TestNormalizeMemberInputsForcesCreatorOwner(t *testing.T) {
	members, err := normalizeMemberInputs("user-1", []MemberInput{
		{UserID: "user-1", Role: RoleMember},
		{UserID: "user-2", Role: RoleAdmin},
	})
	if err != nil {
		t.Fatalf("normalizeMemberInputs() error = %v", err)
	}

	roles := map[string]string{}
	for _, member := range members {
		roles[member.UserID] = member.Role
	}
	if roles["user-1"] != RoleOwner {
		t.Fatalf("creator role = %q, want owner", roles["user-1"])
	}
	if roles["user-2"] != RoleAdmin {
		t.Fatalf("user-2 role = %q, want admin", roles["user-2"])
	}
}

func TestNormalizeMemberInputsRejectsForeignOwner(t *testing.T) {
	_, err := normalizeMemberInputs("user-1", []MemberInput{{UserID: "user-2", Role: RoleOwner}})
	if err == nil {
		t.Fatal("normalizeMemberInputs() error = nil, want owner role error")
	}
	if !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("error = %v, want ErrInvalidArgument", err)
	}
}

func TestCodeForErrorMapsBusinessErrors(t *testing.T) {
	cases := map[error]string{
		ErrInvalidArgument:      CodeInvalidArgument,
		ErrConversationNotFound: CodeConversationNotFound,
		ErrMemberNotFound:       CodeMemberNotFound,
		ErrPermissionDenied:     CodePermissionDenied,
		ErrConversationClosed:   CodeConversationClosed,
		ErrMemberMuted:          CodeMemberMuted,
	}
	for err, want := range cases {
		if got := CodeForError(err); got != want {
			t.Fatalf("CodeForError(%v) = %s, want %s", err, got, want)
		}
	}
}
