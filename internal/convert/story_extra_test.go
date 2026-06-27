package convert

import (
	"testing"

	"github.com/mtgo-labs/mtgo/tg"
)

func TestParseReactionType(t *testing.T) {
	tests := []struct {
		name    string
		in      reactionTypeJSON
		want    func(tg.ReactionClass) bool
		wantErr bool
	}{
		{
			name: "emoji",
			in:   reactionTypeJSON{Type: "emoji", Emoji: "🔥"},
			want: func(r tg.ReactionClass) bool { e, ok := r.(*tg.ReactionEmoji); return ok && e.Emoticon == "🔥" },
		},
		{
			name: "custom_emoji",
			in:   reactionTypeJSON{Type: "custom_emoji", CustomID: "12345"},
			want: func(r tg.ReactionClass) bool { c, ok := r.(*tg.ReactionCustomEmoji); return ok && c.DocumentID == 12345 },
		},
		{
			name: "paid",
			in:   reactionTypeJSON{Type: "paid"},
			want: func(r tg.ReactionClass) bool { _, ok := r.(*tg.ReactionPaid); return ok },
		},
		{
			name:    "bad_custom_id",
			in:      reactionTypeJSON{Type: "custom_emoji", CustomID: "notanumber"},
			wantErr: true,
		},
		{
			name:    "unknown",
			in:      reactionTypeJSON{Type: "mystery"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := parseReactionType(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("error: %v", err)
			}
			if !tt.want(r) {
				t.Errorf("unexpected reaction %T (%+v)", r, r)
			}
		})
	}
}
