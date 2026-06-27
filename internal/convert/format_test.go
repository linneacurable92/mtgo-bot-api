package convert

import (
	"reflect"
	"testing"

	"github.com/mtgo-labs/mtgo/tg"

	apitypes "github.com/mtgo-labs/mtgo-bot-api/internal/types"
)

func TestUTF16Len(t *testing.T) {
	tests := []struct {
		in   string
		want int32
	}{
		{"", 0},
		{"a", 1},
		{"abc", 3},
		// U+1F600 (😀) is one rune but two UTF-16 code units.
		{"😀", 2},
		{"a😀b", 4},
		// Latin-1 supplement (é) fits in one UTF-16 unit.
		{"café", 4},
	}
	for _, tt := range tests {
		if got := utf16Len(tt.in); got != tt.want {
			t.Errorf("utf16Len(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestHTMLTagType(t *testing.T) {
	tests := []struct {
		tag, want string
	}{
		{"b", "bold"},
		{"strong", "bold"},
		{"i", "italic"},
		{"em", "italic"},
		{"u", "underline"},
		{"ins", "underline"},
		{"s", "strikethrough"},
		{"strike", "strikethrough"},
		{"del", "strikethrough"},
		{"tg-spoiler", "spoiler"},
		// Unknown tags pass through unchanged.
		{"code", "code"},
		{"foo", "foo"},
	}
	for _, tt := range tests {
		if got := htmlTagType(tt.tag); got != tt.want {
			t.Errorf("htmlTagType(%q) = %q, want %q", tt.tag, got, tt.want)
		}
	}
}

func TestMarkdownToken(t *testing.T) {
	// Preformatted block triple-backtick wins over single backtick.
	if tok, typ := markdownToken("```x", "MarkdownV2"); tok != "```" || typ != "pre" {
		t.Errorf("triple backtick: tok=%q typ=%q", tok, typ)
	}
	// MarkdownV2 double-star = bold.
	if tok, typ := markdownToken("**x", "MarkdownV2"); tok != "**" || typ != "bold" {
		t.Errorf("mdv2 **: tok=%q typ=%q", tok, typ)
	}
	// Plain markdown has no ** token; the leading * matches the single-star
	// bold rule instead (observed: returns "*", "bold").
	if tok, typ := markdownToken("**x", "markdown"); tok != "*" || typ != "bold" {
		t.Errorf("markdown ** should fall back to single-star bold, got tok=%q typ=%q", tok, typ)
	}
	// MarkdownV2 __ = underline; plain markdown __ = italic.
	if _, typ := markdownToken("__x", "MarkdownV2"); typ != "underline" {
		t.Errorf("mdv2 __ typ=%q want underline", typ)
	}
	if _, typ := markdownToken("__x", "markdown"); typ != "italic" {
		t.Errorf("markdown __ typ=%q want italic", typ)
	}
	// Spoiler and strikethrough are MarkdownV2-only.
	if _, typ := markdownToken("||x", "MarkdownV2"); typ != "spoiler" {
		t.Errorf("mdv2 || typ=%q want spoiler", typ)
	}
	if tok, _ := markdownToken("||x", "markdown"); tok != "" {
		t.Errorf("markdown || should not be a token, got %q", tok)
	}
	if _, typ := markdownToken("~x", "MarkdownV2"); typ != "strikethrough" {
		t.Errorf("mdv2 ~ typ=%q want strikethrough", typ)
	}
	// Single tokens work in both modes.
	if tok, typ := markdownToken("*x", "markdown"); tok != "*" || typ != "bold" {
		t.Errorf("star: tok=%q typ=%q", tok, typ)
	}
	if tok, typ := markdownToken("_x", "markdown"); tok != "_" || typ != "italic" {
		t.Errorf("underscore: tok=%q typ=%q", tok, typ)
	}
	if tok, typ := markdownToken("`x", "markdownv2"); tok != "`" || typ != "code" {
		t.Errorf("backtick: tok=%q typ=%q", tok, typ)
	}
	// No token for ordinary text.
	if tok, typ := markdownToken("hello", "MarkdownV2"); tok != "" || typ != "" {
		t.Errorf("plain text: tok=%q typ=%q want empty", tok, typ)
	}
}

func TestParseHTMLText_SimpleBold(t *testing.T) {
	text, ents, err := parseHTMLText("<b>bold</b>")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if text != "bold" {
		t.Errorf("text = %q, want %q", text, "bold")
	}
	if len(ents) != 1 {
		t.Fatalf("ents = %d, want 1", len(ents))
	}
	be, ok := ents[0].(*tg.MessageEntityBold)
	if !ok {
		t.Fatalf("ent type = %T, want *MessageEntityBold", ents[0])
	}
	if be.Offset != 0 || be.Length != 4 {
		t.Errorf("bold offset/length = %d/%d, want 0/4", be.Offset, be.Length)
	}
}

func TestParseHTMLText_MultipleAndNested(t *testing.T) {
	// Two adjacent spans: <b>bo</b><i>ld</i> -> "bold"
	text, ents, err := parseHTMLText("<b>bo</b><i>ld</i>")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if text != "bold" {
		t.Errorf("text = %q, want bold", text)
	}
	if len(ents) != 2 {
		t.Fatalf("ents = %d, want 2", len(ents))
	}
	// Nested + alias tags: <strong>x<em>y</em>z</strong> -> "xyz"
	text2, ents2, err := parseHTMLText("<strong>x<em>y</em>z</strong>")
	if err != nil {
		t.Fatalf("nested error: %v", err)
	}
	if text2 != "xyz" {
		t.Errorf("nested text = %q, want xyz", text2)
	}
	if len(ents2) != 2 {
		t.Fatalf("nested ents = %d, want 2 (bold + italic)", len(ents2))
	}
}

func TestParseHTMLText_StrikethroughAndSpoiler(t *testing.T) {
	text, ents, err := parseHTMLText("<s>hit</s><tg-spoiler>secret</tg-spoiler>")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if text != "hitsecret" {
		t.Errorf("text = %q, want hitsecret", text)
	}
	if len(ents) != 2 {
		t.Fatalf("ents = %d, want 2", len(ents))
	}
	if _, ok := ents[0].(*tg.MessageEntityStrike); !ok {
		t.Errorf("ent0 type = %T, want Strike", ents[0])
	}
	if _, ok := ents[1].(*tg.MessageEntitySpoiler); !ok {
		t.Errorf("ent1 type = %T, want Spoiler", ents[1])
	}
}

func TestParseHTMLText_PreservesContent(t *testing.T) {
	// Regular text outside tags is preserved, including HTML entities.
	text, _, err := parseHTMLText("a &amp; <b>b</b>")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if text != "a & b" {
		t.Errorf("text = %q, want %q", text, "a & b")
	}
}

func TestParseHTMLText_UnclosedTagError(t *testing.T) {
	if _, _, err := parseHTMLText("<b>no close"); err == nil {
		t.Error("unclosed tag should error")
	}
}

func TestParseHTMLText_StrayCloseTagError(t *testing.T) {
	if _, _, err := parseHTMLText("</b>"); err == nil {
		t.Error("stray close tag should error")
	}
}

func TestParseHTMLText_PreLanguage(t *testing.T) {
	text, ents, err := parseHTMLText(`<pre>code</pre>`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if text != "code" {
		t.Errorf("text = %q, want code", text)
	}
	if len(ents) != 1 {
		t.Fatalf("ents = %d, want 1", len(ents))
	}
	if pe, ok := ents[0].(*tg.MessageEntityPre); !ok || pe.Language != "" {
		t.Errorf("ent = %+v (%T), want pre with empty language", ents[0], ents[0])
	}
}

func TestParseMarkdownText_V2(t *testing.T) {
	text, ents, err := parseMarkdownText("*bold* _italic_ `code`", "MarkdownV2")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if text != "bold italic code" {
		t.Errorf("text = %q, want %q", text, "bold italic code")
	}
	if len(ents) != 3 {
		t.Fatalf("ents = %d, want 3", len(ents))
	}
	if _, ok := ents[0].(*tg.MessageEntityBold); !ok {
		t.Errorf("ent0 = %T, want Bold", ents[0])
	}
	if _, ok := ents[1].(*tg.MessageEntityItalic); !ok {
		t.Errorf("ent1 = %T, want Italic", ents[1])
	}
	if _, ok := ents[2].(*tg.MessageEntityCode); !ok {
		t.Errorf("ent2 = %T, want Code", ents[2])
	}
}

func TestParseMarkdownText_PreAndSpoiler(t *testing.T) {
	text, ents, err := parseMarkdownText("```pre```", "MarkdownV2")
	if err != nil {
		t.Fatalf("pre error: %v", err)
	}
	if text != "pre" || len(ents) != 1 {
		t.Fatalf("pre text=%q ents=%d", text, len(ents))
	}
	if _, ok := ents[0].(*tg.MessageEntityPre); !ok {
		t.Errorf("ent0 = %T, want Pre", ents[0])
	}

	text2, ents2, err := parseMarkdownText("||secret||", "MarkdownV2")
	if err != nil {
		t.Fatalf("spoiler error: %v", err)
	}
	if text2 != "secret" || len(ents2) != 1 {
		t.Fatalf("spoiler text=%q ents=%d", text2, len(ents2))
	}
	if _, ok := ents2[0].(*tg.MessageEntitySpoiler); !ok {
		t.Errorf("ent0 = %T, want Spoiler", ents2[0])
	}
}

func TestParseMarkdownText_Escape(t *testing.T) {
	// Backslash escapes the next char (no entity created).
	text, ents, err := parseMarkdownText(`a\*b`, "MarkdownV2")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if text != "a*b" {
		t.Errorf("text = %q, want a*b", text)
	}
	if len(ents) != 0 {
		t.Errorf("ents = %d, want 0", len(ents))
	}
}

func TestParseMarkdownText_CodeIgnoresFormatting(t *testing.T) {
	// Inside a code span, asterisks are literal.
	text, ents, err := parseMarkdownText("`a*b*c`", "MarkdownV2")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if text != "a*b*c" {
		t.Errorf("text = %q, want a*b*c", text)
	}
	if len(ents) != 1 {
		t.Fatalf("ents = %d, want 1 (code)", len(ents))
	}
}

func TestParseMarkdownText_UnclosedError(t *testing.T) {
	if _, _, err := parseMarkdownText("*bold", "markdown"); err == nil {
		t.Error("unclosed markdown should error")
	}
}

func TestFormattedText_Plain(t *testing.T) {
	text, ents, err := FormattedText("hello", "", "")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if text != "hello" || ents != nil {
		t.Errorf("plain text/ents = %q/%v", text, ents)
	}
}

func TestFormattedText_NoneMode(t *testing.T) {
	if _, _, err := FormattedText("hi", "none", ""); err != nil {
		t.Fatalf("none mode error: %v", err)
	}
}

func TestFormattedText_HTML(t *testing.T) {
	text, ents, err := FormattedText("<b>x</b>", "HTML", "")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if text != "x" || len(ents) != 1 {
		t.Errorf("html text=%q ents=%d", text, len(ents))
	}
}

func TestFormattedText_MarkdownModes(t *testing.T) {
	for _, mode := range []string{"markdown", "Markdown", "markdownv2", "MarkdownV2"} {
		text, ents, err := FormattedText("*x*", mode, "")
		if err != nil {
			t.Errorf("mode %q error: %v", mode, err)
			continue
		}
		if text != "x" {
			t.Errorf("mode %q text = %q, want x", mode, text)
		}
		if len(ents) != 1 {
			t.Errorf("mode %q ents = %d, want 1", mode, len(ents))
		}
	}
}

func TestFormattedText_UnsupportedMode(t *testing.T) {
	if _, _, err := FormattedText("x", "xml", ""); err == nil {
		t.Error("unsupported parse_mode should error")
	}
}

func TestFormattedText_EntitiesJSON(t *testing.T) {
	entitiesJSON := `[{"type":"bold","offset":0,"length":2},{"type":"text_link","offset":3,"length":4,"url":"https://x"}]`
	text, ents, err := FormattedText("plain text", "", entitiesJSON)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if text != "plain text" {
		t.Errorf("text = %q", text)
	}
	if len(ents) != 2 {
		t.Fatalf("ents = %d, want 2", len(ents))
	}
	be, ok := ents[0].(*tg.MessageEntityBold)
	if !ok || be.Offset != 0 || be.Length != 2 {
		t.Errorf("ent0 = %+v (%T)", ents[0], ents[0])
	}
	tl, ok := ents[1].(*tg.MessageEntityTextURL)
	if !ok || tl.URL != "https://x" || tl.Offset != 3 || tl.Length != 4 {
		t.Errorf("ent1 = %+v (%T)", ents[1], ents[1])
	}
}

func TestFormattedText_EntitiesJSONInvalid(t *testing.T) {
	if _, _, err := FormattedText("x", "", "not json"); err == nil {
		t.Error("invalid entities JSON should error")
	}
}

func TestBotAPIEntities_AllTypes(t *testing.T) {
	raw := `[
		{"type":"mention","offset":0,"length":1},
		{"type":"hashtag","offset":0,"length":1},
		{"type":"bot_command","offset":0,"length":1},
		{"type":"url","offset":0,"length":1},
		{"type":"email","offset":0,"length":1},
		{"type":"bold","offset":0,"length":1},
		{"type":"italic","offset":0,"length":1},
		{"type":"code","offset":0,"length":1},
		{"type":"pre","offset":0,"length":1,"language":"go"},
		{"type":"text_link","offset":0,"length":1,"url":"u"},
		{"type":"underline","offset":0,"length":1},
		{"type":"strikethrough","offset":0,"length":1},
		{"type":"spoiler","offset":0,"length":1},
		{"type":"blockquote","offset":0,"length":1},
		{"type":"expandable_blockquote","offset":0,"length":1},
		{"type":"unknown_type","offset":0,"length":1}
	]`
	ents, err := botAPIEntities(raw)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	// Unknown type is dropped -> 15 entities.
	if len(ents) != 15 {
		t.Fatalf("ents = %d, want 15", len(ents))
	}
}

func TestBotAPIEntities_InvalidJSON(t *testing.T) {
	if _, err := botAPIEntities("not json"); err == nil {
		t.Error("invalid JSON should error")
	}
}

func TestBotAPIEntity_PreLanguage(t *testing.T) {
	e := botAPIEntity(apitypes.MessageEntity{Type: "pre", Offset: 1, Length: 2, Language: "go"})
	pe, ok := e.(*tg.MessageEntityPre)
	if !ok {
		t.Fatalf("type = %T, want Pre", e)
	}
	if pe.Language != "go" || pe.Offset != 1 || pe.Length != 2 {
		t.Errorf("pre = %+v", pe)
	}
}

func TestSpansToEntities(t *testing.T) {
	// spansToEntities sorts by offset and converts; unknown types yield nil
	// (dropped from the output).
	spans := []entitySpan{
		{typ: "unknown", offset: 5, length: 1},
		{typ: "bold", offset: 5, length: 2},
		{typ: "italic", offset: 0, length: 3},
	}
	ents := spansToEntities(spans)
	if len(ents) != 2 {
		t.Fatalf("ents = %d, want 2", len(ents))
	}
	// Sorted: italic(0) then bold(5).
	if _, ok := ents[0].(*tg.MessageEntityItalic); !ok {
		t.Errorf("ent0 = %T, want Italic", ents[0])
	}
	if _, ok := ents[1].(*tg.MessageEntityBold); !ok {
		t.Errorf("ent1 = %T, want Bold", ents[1])
	}
}

func TestEntitySpan_Entity_AllTypes(t *testing.T) {
	for typ, want := range map[string]string{
		"bold":          "*tg.MessageEntityBold",
		"italic":        "*tg.MessageEntityItalic",
		"underline":     "*tg.MessageEntityUnderline",
		"strikethrough": "*tg.MessageEntityStrike",
		"code":          "*tg.MessageEntityCode",
		"pre":           "*tg.MessageEntityPre",
		"spoiler":       "*tg.MessageEntitySpoiler",
	} {
		s := entitySpan{typ: typ, offset: 1, length: 2}
		got := s.entity()
		if got == nil {
			t.Errorf("%s: nil entity", typ)
			continue
		}
		if reflect.TypeOf(got).String() != want {
			t.Errorf("%s: type = %T, want %s", typ, got, want)
		}
	}
	// Unknown type -> nil.
	if (&entitySpan{typ: "nope"}).entity() != nil {
		t.Error("unknown type should return nil")
	}
}
