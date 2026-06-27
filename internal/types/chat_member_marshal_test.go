package types

import (
	"encoding/json"
	"strings"
	"testing"
)

// hasKey reports whether the JSON object output contains the given key.
func hasKey(s, key string) bool {
	return strings.Contains(s, `"`+key+`":`)
}

func mkUser() *User { return &User{ID: 42, FirstName: "Alice"} }

// TestSetChatType verifies the setter records the chat type and that MarshalJSON
// honors it (channel gating emits can_post_messages that supergroup does not).
func TestSetChatType(t *testing.T) {
	admin := ChatMember{User: mkUser(), Status: "administrator", CanPostMessages: true}

	admin.SetChatType(ChatTypeSupergroup)
	b, _ := json.Marshal(admin)
	if hasKey(string(b), "can_post_messages") {
		t.Errorf("supergroup admin must NOT emit can_post_messages; got %s", b)
	}

	admin.SetChatType(ChatTypeChannel)
	b, _ = json.Marshal(admin)
	if !hasKey(string(b), "can_post_messages") {
		t.Errorf("channel admin MUST emit can_post_messages; got %s", b)
	}
}

// TestChatMemberMarshal_Statuses exercises every switch branch in MarshalJSON
// (creator, administrator, member, restricted, kicked) plus the nil-User path.
func TestChatMemberMarshal_Statuses(t *testing.T) {
	cases := []struct {
		name    string
		member  ChatMember
		chatT   ChatType
		wantHas []string
		wantNot []string
	}{
		{
			name:    "creator with custom title",
			member:  ChatMember{User: mkUser(), Status: "creator", CustomTitle: "Boss", IsAnonymous: true},
			chatT:   ChatTypeSupergroup,
			wantHas: []string{"custom_title", "is_anonymous"},
		},
		{
			name:    "creator without custom title",
			member:  ChatMember{User: mkUser(), Status: "creator"},
			chatT:   ChatTypeSupergroup,
			wantNot: []string{"custom_title"},
			wantHas: []string{"is_anonymous"},
		},
		{
			name:    "administrator channel gating",
			member:  ChatMember{User: mkUser(), Status: "administrator", CanPostMessages: true, CanEditMessages: true, CanManageDirectMessages: true, CanManageTags: true, CanPinMessages: true, CanManageTopics: true, CustomTitle: "Op"},
			chatT:   ChatTypeChannel,
			wantHas: []string{"can_be_edited", "can_post_messages", "can_edit_messages", "can_manage_direct_messages", "can_manage_voice_chats", "custom_title"},
			wantNot: []string{"can_pin_messages", "can_manage_topics", "can_manage_tags"},
		},
		{
			name:    "administrator group gating",
			member:  ChatMember{User: mkUser(), Status: "administrator", CanPinMessages: true, CanManageTags: true},
			chatT:   ChatTypeGroup,
			wantHas: []string{"can_be_edited", "can_pin_messages", "can_manage_tags"},
			wantNot: []string{"can_post_messages", "can_manage_topics", "can_manage_direct_messages"},
		},
		{
			name:    "administrator supergroup gating",
			member:  ChatMember{User: mkUser(), Status: "administrator", CanManageTopics: true},
			chatT:   ChatTypeSupergroup,
			wantHas: []string{"can_be_edited", "can_pin_messages", "can_manage_topics", "can_manage_tags"},
			wantNot: []string{"can_post_messages", "can_manage_direct_messages"},
		},
		{
			name:    "administrator private (default) no gating",
			member:  ChatMember{User: mkUser(), Status: "administrator"},
			chatT:   ChatTypePrivate,
			wantHas: []string{"can_be_edited", "can_manage_chat"},
			wantNot: []string{"can_post_messages", "can_pin_messages", "can_manage_topics", "can_manage_direct_messages", "can_manage_tags"},
		},
		{
			name:    "administrator default chatType (empty -> supergroup)",
			member:  ChatMember{User: mkUser(), Status: "administrator", CanManageTopics: true},
			chatT:   "", // exercises the ct=="" -> ChatTypeSupergroup default
			wantHas: []string{"can_manage_topics"},
		},
		{
			name:    "member with tag and until_date",
			member:  ChatMember{User: mkUser(), Status: "member", Tag: "vip", UntilDate: 100},
			chatT:   ChatTypeSupergroup,
			wantHas: []string{"tag", "until_date"},
		},
		{
			name:    "member plain",
			member:  ChatMember{User: mkUser(), Status: "member"},
			chatT:   ChatTypeSupergroup,
			wantNot: []string{"tag", "until_date"},
		},
		{
			name:    "restricted supergroup emits permissions",
			member:  ChatMember{User: mkUser(), Status: "restricted", Tag: "r", UntilDate: 200, IsMember: true, CanSendMessages: true, CanEditTag: true},
			chatT:   ChatTypeSupergroup,
			wantHas: []string{"tag", "until_date", "is_member", "can_send_messages", "can_edit_tag", "can_manage_topics"},
		},
		{
			name:    "restricted non-supergroup omits permissions",
			member:  ChatMember{User: mkUser(), Status: "restricted", CanSendMessages: true},
			chatT:   ChatTypeGroup,
			wantNot: []string{"can_send_messages", "until_date", "is_member"},
		},
		{
			name:    "kicked emits until_date",
			member:  ChatMember{User: mkUser(), Status: "kicked", UntilDate: 300},
			chatT:   ChatTypeSupergroup,
			wantHas: []string{"until_date"},
		},
		{
			name:    "nil user omits user key",
			member:  ChatMember{Status: "member"},
			chatT:   ChatTypeSupergroup,
			wantNot: []string{"user"},
			wantHas: []string{"status"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := tc.member
			m.SetChatType(tc.chatT)
			b, err := json.Marshal(m)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			s := string(b)
			for _, k := range tc.wantHas {
				if !hasKey(s, k) {
					t.Errorf("expected key %q; got %s", k, s)
				}
			}
			for _, k := range tc.wantNot {
				if hasKey(s, k) {
					t.Errorf("did not expect key %q; got %s", k, s)
				}
			}
		})
	}
}

// TestAppendChatAdminRights_Direct calls the helper directly across chat types
// to guarantee full branch coverage independent of the MarshalJSON wiring.
func TestAppendChatAdminRights_Direct(t *testing.T) {
	m := ChatMember{CanPostMessages: true, CanManageDirectMessages: true, CanManageTags: true, CanManageTopics: true, CanPinMessages: true}
	for _, ct := range []ChatType{ChatTypeChannel, ChatTypeGroup, ChatTypeSupergroup, ChatTypePrivate} {
		f := appendChatAdminRights(nil, m, ct)
		if len(f) == 0 {
			t.Errorf("chatType %s: expected fields, got none", ct)
		}
		// marshal to validate the jsonField slice round-trips through marshalOrdered.
		b, err := marshalOrdered(f)
		if err != nil {
			t.Errorf("chatType %s: marshalOrdered: %v", ct, err)
		}
		if !strings.HasPrefix(string(b), "{") {
			t.Errorf("chatType %s: expected JSON object, got %s", ct, b)
		}
	}
}

// TestAppendChatPermissions_Direct calls the helper directly.
func TestAppendChatPermissions_Direct(t *testing.T) {
	m := ChatMember{CanSendMessages: true, CanEditTag: true}
	f := appendChatPermissions(nil, m)
	if len(f) == 0 {
		t.Fatal("expected permission fields, got none")
	}
	b, err := marshalOrdered(f)
	if err != nil {
		t.Fatalf("marshalOrdered: %v", err)
	}
	s := string(b)
	for _, k := range []string{"can_send_messages", "can_edit_tag", "can_manage_topics"} {
		if !hasKey(s, k) {
			t.Errorf("expected key %q; got %s", k, s)
		}
	}
}
