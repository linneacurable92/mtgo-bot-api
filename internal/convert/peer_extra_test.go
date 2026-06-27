package convert

import (
	"context"
	"testing"

	"github.com/mtgo-labs/mtgo/tg"

	"github.com/mtgo-labs/mtgo-bot-api/internal/storage"
)

// --- peerToInputPeer (pure) ------------------------------------------------

func TestPeerToInputPeer(t *testing.T) {
	tests := []struct {
		name    string
		peer    storage.Peer
		want    func(tg.InputPeerClass) bool
		wantErr bool
	}{
		{
			name: "user",
			peer: storage.Peer{ID: 1, AccessHash: 11, Type: storage.PeerTypeUser},
			want: func(p tg.InputPeerClass) bool { _, ok := p.(*tg.InputPeerUser); return ok },
		},
		{
			name: "chat",
			peer: storage.Peer{ID: 2, Type: storage.PeerTypeChat},
			want: func(p tg.InputPeerClass) bool { _, ok := p.(*tg.InputPeerChat); return ok },
		},
		{
			name: "channel",
			peer: storage.Peer{ID: 3, AccessHash: 33, Type: storage.PeerTypeChannel},
			want: func(p tg.InputPeerClass) bool { _, ok := p.(*tg.InputPeerChannel); return ok },
		},
		{
			name:    "unknown",
			peer:    storage.Peer{ID: 9, Type: "bogus"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := peerToInputPeer(tt.peer)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("error: %v", err)
			}
			if !tt.want(p) {
				t.Errorf("unexpected type %T", p)
			}
		})
	}
}

func TestPeerToInputPeer_UserAccessHash(t *testing.T) {
	p, err := peerToInputPeer(storage.Peer{ID: 5, AccessHash: 77, Type: storage.PeerTypeUser})
	if err != nil {
		t.Fatal(err)
	}
	u, ok := p.(*tg.InputPeerUser)
	if !ok || u.AccessHash != 77 {
		t.Errorf("user peer = %+v", p)
	}
}

func TestPeerToInputPeer_ChannelAccessHash(t *testing.T) {
	p, err := peerToInputPeer(storage.Peer{ID: 5, AccessHash: 88, Type: storage.PeerTypeChannel})
	if err != nil {
		t.Fatal(err)
	}
	c, ok := p.(*tg.InputPeerChannel)
	if !ok || c.AccessHash != 88 {
		t.Errorf("channel peer = %+v", p)
	}
}

// --- ResolvePeer (nil-store branches) --------------------------------------

func TestResolvePeer_NilStore(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name    string
		chatID  string
		want    func(tg.InputPeerClass) bool
		wantErr bool
	}{
		{"empty", "", nil, true},
		{"at_no_store", "@alice", nil, true}, // nil store → cache miss error
		{"invalid", "abc", nil, true},        // not int, not @user
		{"zero", "0", nil, true},             // id==0 → chat not found
		{"user", "123", func(p tg.InputPeerClass) bool {
			u, ok := p.(*tg.InputPeerUser)
			return ok && u.UserID == 123
		}, false},
		{"legacy_chat", "-456", func(p tg.InputPeerClass) bool {
			c, ok := p.(*tg.InputPeerChat)
			return ok && c.ChatID == 456
		}, false},
		{"channel", "-100789", func(p tg.InputPeerClass) bool {
			c, ok := p.(*tg.InputPeerChannel)
			return ok && c.ChannelID == 789
		}, false},
		{"channel_zero_suffix", "-100", nil, true}, // channelID<=0 → error
		{"at_empty", "@", nil, true},               // empty username
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := ResolvePeer(ctx, tt.chatID, nil)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("error: %v", err)
			}
			if !tt.want(p) {
				t.Errorf("unexpected result %T (%+v)", p, p)
			}
		})
	}
}

// newTestStore opens a throwaway per-bot store in a temp dir.
func newTestStore(t *testing.T) *storage.Store {
	t.Helper()
	s, err := storage.Open(t.TempDir(), "1")
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestResolvePeer_StoreUserHit(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	if err := store.SavePeer(ctx, storage.Peer{ID: 555, AccessHash: 42, Type: storage.PeerTypeUser}); err != nil {
		t.Fatal(err)
	}
	p, err := ResolvePeer(ctx, "555", store)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	u, ok := p.(*tg.InputPeerUser)
	if !ok || u.UserID != 555 || u.AccessHash != 42 {
		t.Errorf("resolved user peer = %+v", p)
	}
}

func TestResolvePeer_StoreChannelHit(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	if err := store.SavePeer(ctx, storage.Peer{ID: 9, AccessHash: 7, Type: storage.PeerTypeChannel}); err != nil {
		t.Fatal(err)
	}
	// Channel chat_id = -1000000000000 - 9 = -1000000000009.
	p, err := ResolvePeer(ctx, "-1000000000009", store)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	c, ok := p.(*tg.InputPeerChannel)
	if !ok || c.ChannelID != 9 || c.AccessHash != 7 {
		t.Errorf("resolved channel peer = %+v", p)
	}
}

func TestResolvePeer_StoreUsernameHit(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	if err := store.SavePeer(ctx, storage.Peer{
		ID: 8, AccessHash: 9, Type: storage.PeerTypeUser, Username: "bob",
	}); err != nil {
		t.Fatal(err)
	}
	p, err := ResolvePeer(ctx, "@bob", store)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	u, ok := p.(*tg.InputPeerUser)
	if !ok || u.UserID != 8 || u.AccessHash != 9 {
		t.Errorf("resolved username peer = %+v", p)
	}
}
