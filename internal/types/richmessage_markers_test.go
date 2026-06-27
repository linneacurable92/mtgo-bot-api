package types

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestRichBlockMarkers calls every isRichBlock marker through the interface
// (which dispatches to and executes the concrete method body) and marshals each
// concrete block. This lifts the ~21 0%-coverage marker methods plus their JSON
// emission paths.
func TestRichBlockMarkers(t *testing.T) {
	blocks := []RichBlock{
		&RichBlockParagraph{Type: "paragraph", Text: RichTextPlain("p")},
		&RichBlockSectionHeading{Type: "heading", Text: RichTextPlain("h"), Size: 2},
		&RichBlockPreformatted{Type: "pre", Text: RichTextPlain("code"), Language: "go"},
		&RichBlockFooter{Type: "footer", Text: RichTextPlain("f")},
		&RichBlockDivider{Type: "divider"},
		&RichBlockMathematicalExpression{Type: "mathematical_expression", Expression: "a+b"},
		&RichBlockAnchor{Type: "anchor", Name: "sec1"},
		&RichBlockList{Type: "list", Items: []RichBlockListItem{{Label: "x", Type: "ordered"}}},
		&RichBlockBlockQuotation{Type: "blockquote", Blocks: []RichBlock{&RichBlockParagraph{Type: "paragraph", Text: RichTextPlain("q")}}},
		&RichBlockPullQuotation{Type: "pullquote", Text: RichTextPlain("pq"), Credit: RichTextPlain("c")},
		&RichBlockCollage{Type: "collage", Blocks: []RichBlock{&RichBlockPhoto{Type: "photo", Photo: &PhotoSize{FileID: "1"}}}},
		&RichBlockSlideshow{Type: "slideshow", Blocks: []RichBlock{&RichBlockPhoto{Type: "photo", Photo: &PhotoSize{FileID: "2"}}}},
		&RichBlockTable{Type: "table", Cells: [][]RichTableCell{{}}},
		&RichBlockDetails{Type: "details", Summary: RichTextPlain("s"), Blocks: []RichBlock{&RichBlockParagraph{Type: "paragraph", Text: RichTextPlain("d")}}, IsOpen: true},
		&RichBlockMap{Type: "map", Location: &Location{Latitude: 1, Longitude: 2}, Zoom: 10, Width: 100, Height: 100},
		&RichBlockAnimation{Type: "animation", Animation: &Animation{FileID: "a"}, NeedAutoplay: true, HasSpoiler: true},
		&RichBlockAudio{Type: "audio", Audio: &Audio{FileID: "au"}},
		&RichBlockPhoto{Type: "photo", Photo: &PhotoSize{FileID: "ph"}, HasSpoiler: true},
		&RichBlockVideo{Type: "video", Video: &Video{FileID: "v"}, NeedAutoplay: true, IsLooped: true, HasSpoiler: true},
		&RichBlockVoiceNote{Type: "voice_note", VoiceNote: &Voice{FileID: "vn"}},
		&RichBlockThinking{Type: "thinking"},
	}

	for i, b := range blocks {
		// Calling through the interface executes the concrete isRichBlock body.
		b.isRichBlock()
		data, err := json.Marshal(b)
		if err != nil {
			t.Errorf("block[%d] (%T) marshal error: %v", i, b, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("block[%d] (%T) produced empty JSON", i, b)
		}
	}

	// compile time via the slice assignment above; assert the count is complete.
	want := 21
	if len(blocks) != want {
		t.Fatalf("expected %d rich blocks, got %d (new block type added?)", want, len(blocks))
	}
}

// TestRichTextMarkers calls every isRichText marker through the interface and
// marshals each concrete rich-text value. Lifts the ~27 0%-coverage markers.
func TestRichTextMarkers(t *testing.T) {
	texts := []RichText{
		RichTextPlain("plain"),
		RichTexts{RichTextPlain("a"), RichTextPlain("b")},
		&RichTextBold{Type: "bold", Text: RichTextPlain("b")},
		&RichTextItalic{Type: "italic", Text: RichTextPlain("i")},
		&RichTextUnderline{Type: "underline", Text: RichTextPlain("u")},
		&RichTextStrikethrough{Type: "strikethrough", Text: RichTextPlain("s")},
		&RichTextSpoiler{Type: "spoiler", Text: RichTextPlain("sp")},
		&RichTextDateTime{Type: "date_time", Text: RichTextPlain("now"), UnixTime: 1700000000, DateTimeFormat: "iso"},
		&RichTextTextMention{Type: "text_mention", Text: RichTextPlain("user"), User: &User{ID: 7, FirstName: "Bob"}},
		&RichTextSubscript{Type: "subscript", Text: RichTextPlain("sub")},
		&RichTextSuperscript{Type: "superscript", Text: RichTextPlain("sup")},
		&RichTextMarked{Type: "marked", Text: RichTextPlain("m")},
		&RichTextCode{Type: "code", Text: RichTextPlain("c")},
		&RichTextCustomEmoji{Type: "custom_emoji", CustomEmojiID: "5", AlternativeText: "alt"},
		&RichTextMathematicalExpression{Type: "mathematical_expression", Expression: "x^2"},
		&RichTextURL{Type: "url", Text: RichTextPlain("lnk"), URL: "https://e.com"},
		&RichTextEmailAddress{Type: "email_address", Text: RichTextPlain("mail"), EmailAddress: "a@b.com"},
		&RichTextPhoneNumber{Type: "phone_number", Text: RichTextPlain("tel"), PhoneNumber: "+1"},
		&RichTextBankCardNumber{Type: "bank_card_number", Text: RichTextPlain("cc"), BankCardNumber: "4242"},
		&RichTextMention{Type: "mention", Text: RichTextPlain("men"), Username: "user"},
		&RichTextHashtag{Type: "hashtag", Text: RichTextPlain("hash"), Hashtag: "#tag"},
		&RichTextCashtag{Type: "cashtag", Text: RichTextPlain("cash"), Cashtag: "$USD"},
		&RichTextBotCommand{Type: "bot_command", Text: RichTextPlain("cmd"), BotCommand: "/start"},
		&RichTextAnchor{Type: "anchor", Name: "a1"},
		&RichTextAnchorLink{Type: "anchor_link", Text: RichTextPlain("al"), AnchorName: "a1"},
		&RichTextReference{Type: "reference", Text: RichTextPlain("ref"), Name: "n"},
		&RichTextReferenceLink{Type: "reference_link", Text: RichTextPlain("rl"), ReferenceName: "rn"},
	}

	for i, rt := range texts {
		// Calling through the interface executes the concrete isRichText body.
		rt.isRichText()
		data, err := json.Marshal(rt)
		if err != nil {
			t.Errorf("text[%d] (%T) marshal error: %v", i, rt, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("text[%d] (%T) produced empty JSON", i, rt)
		}
	}

	want := 27
	if len(texts) != want {
		t.Fatalf("expected %d rich texts, got %d (new text type added?)", want, len(texts))
	}
}

// roundTrip marshals v then unmarshals into a fresh value of the same type and
// compares the re-marshaled bytes. Used for rich types whose fields are all
// plain (no RichText/RichBlock interface fields, which json cannot unmarshal).
func roundTrip[T any](t *testing.T, v T) {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal %T: %v", v, err)
	}
	var got T
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal %T (%s): %v", v, raw, err)
	}
	raw2, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("re-marshal %T: %v", v, err)
	}
	if string(raw) != string(raw2) {
		t.Errorf("%T round-trip mismatch:\n in:  %s\n out: %s", v, raw, raw2)
	}
}

// TestRichTypes_RoundTrip verifies JSON marshal/unmarshal fidelity for the rich
// types whose fields are all plain values (interfaces cannot be unmarshaled).
func TestRichTypes_RoundTrip(t *testing.T) {
	cases := []any{
		// Rich blocks with plain fields.
		RichBlockDivider{Type: "divider"},
		RichBlockMathematicalExpression{Type: "mathematical_expression", Expression: "a+b"},
		RichBlockAnchor{Type: "anchor", Name: "sec1"},
		RichBlockThinking{Type: "thinking"},
		RichBlockListItem{Label: "x", Type: "ordered", Value: "v", HasCheckbox: true, IsChecked: true},
		// Rich text with plain fields.
		RichTextCustomEmoji{Type: "custom_emoji", CustomEmojiID: "5", AlternativeText: "alt"},
		RichTextMathematicalExpression{Type: "mathematical_expression", Expression: "x^2"},
		RichTextAnchor{Type: "anchor", Name: "a1"},
	}
	for _, c := range cases {
		switch v := c.(type) {
		case RichBlockDivider:
			roundTrip(t, v)
		case RichBlockMathematicalExpression:
			roundTrip(t, v)
		case RichBlockAnchor:
			roundTrip(t, v)
		case RichBlockThinking:
			roundTrip(t, v)
		case RichBlockListItem:
			roundTrip(t, v)
		case RichTextCustomEmoji:
			roundTrip(t, v)
		case RichTextMathematicalExpression:
			roundTrip(t, v)
		case RichTextAnchor:
			roundTrip(t, v)
		default:
			t.Fatalf("unhandled round-trip case %T", c)
		}
	}
}

// TestRichTableCell_JSON exercises the RichTableCell marshal path (used inside
// RichBlockTable) across header/span/align combinations.
func TestRichTableCell_JSON(t *testing.T) {
	cells := []RichTableCell{
		{Text: RichTextPlain("c"), Align: "left", Valign: "top"},
		{Text: RichTextPlain("h"), IsHeader: true, Colspan: 2, Rowspan: 1, Align: "center", Valign: "middle"},
	}
	for i, c := range cells {
		data, err := json.Marshal(c)
		if err != nil {
			t.Errorf("cell[%d] marshal error: %v", i, err)
			continue
		}
		if !strings.Contains(string(data), `"align"`) {
			t.Errorf("cell[%d] missing align; got %s", i, data)
		}
	}
}

// TestRichBlockCaption_JSON exercises the caption helper struct (Text + Credit).
func TestRichBlockCaption_JSON(t *testing.T) {
	c := RichBlockCaption{Text: RichTextPlain("cap"), Credit: RichTextPlain("by")}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(data), `"text"`) || !strings.Contains(string(data), `"credit"`) {
		t.Errorf("caption missing fields; got %s", data)
	}
}
