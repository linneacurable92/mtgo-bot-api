package convert

import (
	"testing"

	"github.com/mtgo-labs/mtgo/tg"

	apitypes "github.com/mtgo-labs/mtgo-bot-api/internal/types"
)

func TestConvertInlineButton_AllVariants(t *testing.T) {
	tests := []struct {
		name   string
		btn    apitypes.InlineKeyboardButton
		want   func(tg.KeyboardButtonClass) bool
	}{
		{
			name: "callback_data",
			btn:  apitypes.InlineKeyboardButton{Text: "T", CallbackData: "cd"},
			want: func(b tg.KeyboardButtonClass) bool { _, ok := b.(*tg.KeyboardButtonCallback); return ok },
		},
		{
			name: "url",
			btn:  apitypes.InlineKeyboardButton{Text: "T", URL: "https://x"},
			want: func(b tg.KeyboardButtonClass) bool { _, ok := b.(*tg.KeyboardButtonURL); return ok },
		},
		{
			name: "web_app",
			btn:  apitypes.InlineKeyboardButton{Text: "T", WebApp: &apitypes.WebAppInfo{URL: "https://app"}},
			want: func(b tg.KeyboardButtonClass) bool { _, ok := b.(*tg.KeyboardButtonWebView); return ok },
		},
		{
			name: "switch_chosen_chat",
			btn:  apitypes.InlineKeyboardButton{Text: "T", SwitchInlineQueryChosenChat: &apitypes.SwitchInlineQueryChosenChat{Query: "q"}},
			want: func(b tg.KeyboardButtonClass) bool { cb, ok := b.(*tg.KeyboardButtonSwitchInline); return ok && !cb.SamePeer },
		},
		{
			name: "switch_current_chat",
			btn:  apitypes.InlineKeyboardButton{Text: "T", SwitchInlineQueryCurrentChat: "cur"},
			want: func(b tg.KeyboardButtonClass) bool { cb, ok := b.(*tg.KeyboardButtonSwitchInline); return ok && cb.SamePeer && cb.Query == "cur" },
		},
		{
			name: "switch_inline_query",
			btn:  apitypes.InlineKeyboardButton{Text: "T", SwitchInlineQuery: "all"},
			want: func(b tg.KeyboardButtonClass) bool { cb, ok := b.(*tg.KeyboardButtonSwitchInline); return ok && cb.Query == "all" && !cb.SamePeer },
		},
		{
			name: "pay",
			btn:  apitypes.InlineKeyboardButton{Text: "T", Pay: true},
			want: func(b tg.KeyboardButtonClass) bool { _, ok := b.(*tg.KeyboardButtonBuy); return ok },
		},
		{
			name: "default",
			btn:  apitypes.InlineKeyboardButton{Text: "T"},
			want: func(b tg.KeyboardButtonClass) bool { _, ok := b.(*tg.KeyboardButton); return ok },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertInlineButton(tt.btn)
			if !tt.want(got) {
				t.Errorf("unexpected type %T (%+v)", got, got)
			}
		})
	}
}

func TestConvertInlineButton_CallbackDataValue(t *testing.T) {
	b := convertInlineButton(apitypes.InlineKeyboardButton{Text: "Go", CallbackData: "payload"})
	cb, ok := b.(*tg.KeyboardButtonCallback)
	if !ok {
		t.Fatalf("type = %T", b)
	}
	if cb.Text != "Go" || string(cb.Data) != "payload" {
		t.Errorf("callback = %+v", cb)
	}
}

func TestReplyMarkup_FullMarkup(t *testing.T) {
	// ReplyMarkup now also exercises convertInlineButton via the public path.
	raw := `{"inline_keyboard":[[{"text":"A","callback_data":"x"},{"text":"B","url":"https://b"}]]}`
	rm, err := ReplyMarkup(raw)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	inline, ok := rm.(*tg.ReplyInlineMarkup)
	if !ok {
		t.Fatalf("type = %T, want *ReplyInlineMarkup", rm)
	}
	if len(inline.Rows) != 1 || len(inline.Rows[0].Buttons) != 2 {
		t.Fatalf("rows/buttons = %+v", inline.Rows)
	}
	if _, ok := inline.Rows[0].Buttons[0].(*tg.KeyboardButtonCallback); !ok {
		t.Errorf("button0 type = %T", inline.Rows[0].Buttons[0])
	}
}

func TestReplyMarkup_InvalidJSON(t *testing.T) {
	if _, err := ReplyMarkup("{bad}"); err == nil {
		t.Error("invalid JSON should error")
	}
}
