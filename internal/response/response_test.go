package response

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestOKContainsResult(t *testing.T) {
	b := OK(map[string]any{"id": 42.0}, "done")
	var env struct {
		Ok          bool            `json:"ok"`
		Result      json.RawMessage `json:"result"`
		Description string          `json:"description"`
	}
	if err := json.Unmarshal(b, &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !env.Ok {
		t.Errorf("ok = false, want true (body=%s)", b)
	}
	if strings.TrimSpace(string(env.Result)) == "" {
		t.Errorf("result missing on success (body=%s)", b)
	}
	if env.Description != "done" {
		t.Errorf("description = %q, want %q", env.Description, "done")
	}
}

func TestOKAlwaysIncludesResultEvenForFalse(t *testing.T) {
	// No Bot API success returns false, but the envelope contract must still
	// carry result; verify a boolean false is not dropped.
	b := OK(false, "")
	if !strings.Contains(string(b), `"result":false`) {
		t.Errorf("result:false dropped from envelope (body=%s)", b)
	}
}

func TestFailShape(t *testing.T) {
	b := Fail(400, "Bad Request: nope", &Parameters{RetryAfter: 5})
	var env struct {
		Ok          bool `json:"ok"`
		ErrorCode   int  `json:"error_code"`
		Parameters  *Parameters
		Description string `json:"description"`
	}
	if err := json.Unmarshal(b, &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.Ok {
		t.Errorf("ok = true on failure")
	}
	if env.ErrorCode != 400 {
		t.Errorf("error_code = %d, want 400", env.ErrorCode)
	}
	if env.Description != "Bad Request: nope" {
		t.Errorf("description = %q", env.Description)
	}
	if env.Parameters == nil || env.Parameters.RetryAfter != 5 {
		t.Errorf("parameters not carried: %+v", env.Parameters)
	}
}

func TestFailHasNoResult(t *testing.T) {
	b := Fail(404, "not found", nil)
	if strings.Contains(string(b), `"result"`) {
		t.Errorf("error envelope must not include result (body=%s)", b)
	}
}

func TestDecode(t *testing.T) {
	ok, _, code, desc, err := Decode(OK(7, ""))
	if err != nil || !ok || code != 0 || desc != "" {
		t.Errorf("Decode success mismatch: ok=%v code=%d desc=%q err=%v", ok, code, desc, err)
	}
	ok, _, code, _, err = Decode(Fail(429, "x", nil))
	if err != nil || ok || code != 429 {
		t.Errorf("Decode error mismatch: ok=%v code=%d err=%v", ok, code, err)
	}
}

func TestEscapeNonASCII(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want string
	}{
		// Structural bytes outside string values pass through unchanged.
		{"non-string bytes untouched", []byte(`{"n":42,"arr":[1,2]}`), `{"n":42,"arr":[1,2]}`},
		{"pure ASCII passthrough", []byte(`{"k":"abc 123"}`), `{"k":"abc 123"}`},
		// Latin-1 accented: é = U+00E9 (UTF-8 c3 a9) → BMP \u00e9.
		{"latin1 accented in string", []byte("\"caf\xc3\xa9\""), `"caf\u00e9"`},
		// Cyrillic: й = U+0439 (UTF-8 d0 b9) → BMP \u0439.
		{"cyrillic in string", []byte("\"\xd0\xb9\""), `"\u0439"`},
		// CJK: 中 = U+4E2D (UTF-8 e4 b8 ad) → BMP \u4e2d.
		{"cjk in string", []byte("\"\xe4\xb8\xad\""), `"\u4e2d"`},
		// Supplementary plane: 😀 = U+1F600 (UTF-8 f0 9f 98 80) → surrogate pair.
		{"emoji surrogate pair", []byte("\"\xf0\x9f\x98\x80\""), `"\ud83d\ude00"`},
		// Invalid UTF-8 lead byte 0xff → RuneError → \ufffd.
		{"invalid utf-8 becomes replacement", []byte("\"\xff\""), `"\ufffd"`},
		// Existing escape sequences (Go's \u003c for '<') are copied verbatim.
		{"backslash escape sequence preserved", []byte(`"a\u003cb"`), `"a\u003cb"`},
		// Trailing backslash at end of data: next-byte guard skips it.
		{"trailing backslash at end", []byte("\"x\\"), `"x\`},
		// Mixed ASCII, BMP, and supplementary plane runes.
		{"mixed ascii and non-ascii", []byte("\"hi \xc3\xa9\xf0\x9f\x98\x80!\""), `"hi \u00e9\ud83d\ude00!"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(escapeNonASCII(tt.in))
			if got != tt.want {
				t.Errorf("escapeNonASCII(%q) =\n  %q\nwant\n  %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestOKEscapesNonASCII(t *testing.T) {
	// A result containing non-ASCII forces the escape path through OK.
	b := OK(map[string]string{"name": "caf\xc3\xa9\xf0\x9f\x98\x80"}, "")
	if !strings.Contains(string(b), `"name":"caf\u00e9\ud83d\ude00"`) {
		t.Fatalf("non-ASCII not escaped in OK output: %s", b)
	}
	// The escaped output must still be valid JSON that round-trips to the
	// original Unicode text.
	var env struct {
		Ok     bool                   `json:"ok"`
		Result map[string]string      `json:"result"`
	}
	if err := json.Unmarshal(b, &env); err != nil {
		t.Fatalf("escaped OK output is not valid JSON: %v (body=%s)", err, b)
	}
	if !env.Ok {
		t.Errorf("ok = false, want true (body=%s)", b)
	}
	if env.Result["name"] != "caf\xc3\xa9\xf0\x9f\x98\x80" {
		t.Errorf("round-trip name = %q", env.Result["name"])
	}
}

func TestOKFallbackOnMarshalError(t *testing.T) {
	// A channel cannot be JSON-marshalled, exercising the fallback path
	// that guarantees a valid minimal envelope is always returned.
	b := OK(make(chan int), "")
	if string(b) != `{"ok":true,"result":null}` {
		t.Errorf("marshal-error fallback = %s, want {\"ok\":true,\"result\":null}", b)
	}
}

func TestFailEscapesNonASCII(t *testing.T) {
	// A description containing non-ASCII forces the escape path through Fail.
	b := Fail(400, "Bad Request: caf\xc3\xa9", nil)
	if !strings.Contains(string(b), `caf\u00e9`) {
		t.Errorf("non-ASCII not escaped in Fail output: %s", b)
	}
	// Verify it still round-trips to the original text.
	ok, _, code, desc, err := Decode(b)
	if err != nil || ok || code != 400 {
		t.Fatalf("Decode mismatch: ok=%v code=%d err=%v (body=%s)", ok, code, err, b)
	}
	if desc != "Bad Request: caf\xc3\xa9" {
		t.Errorf("round-trip description = %q (body=%s)", desc, b)
	}
}

func TestFailWithAndWithoutParameters(t *testing.T) {
	// With parameters: the parameters object must appear in the output.
	withParams := Fail(429, "rate limited", &Parameters{MigrateToChatID: 7, RetryAfter: 3})
	if !strings.Contains(string(withParams), `"migrate_to_chat_id":7`) ||
		!strings.Contains(string(withParams), `"retry_after":3`) {
		t.Errorf("parameters missing from Fail output: %s", withParams)
	}
	// Without parameters: the parameters key must be omitted entirely.
	noParams := Fail(404, "not found", nil)
	if strings.Contains(string(noParams), `"parameters"`) {
		t.Errorf("parameters key should be omitted when nil: %s", noParams)
	}
}
