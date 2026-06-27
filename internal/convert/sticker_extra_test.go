package convert

import (
	"testing"

	"github.com/mtgo-labs/mtgo/tg"
)

// --- StickerFromDocument ----------------------------------------------------

func TestStickerFromDocument_Nil(t *testing.T) {
	if got := StickerFromDocument(nil, nil, ""); got != nil {
		t.Error("nil document should return nil")
	}
}

func TestStickerFromDocument_Regular(t *testing.T) {
	doc := &tg.Document{
		ID:         100,
		AccessHash: 200,
		DCID:       2,
		Size:       1024,
		MimeType:   "image/webp",
		Attributes: []tg.DocumentAttributeClass{
			&tg.DocumentAttributeImageSize{W: 512, H: 512},
			&tg.DocumentAttributeSticker{
				Alt:        "😀",
				Stickerset: &tg.InputStickerSetShortName{ShortName: "MySet"},
			},
		},
		Thumbs: []tg.PhotoSizeClass{
			&tg.PhotoSize{Type: "s", W: 128, H: 128, Size: 4096},
		},
	}
	s := StickerFromDocument(doc, nil, "")
	if s == nil {
		t.Fatal("nil sticker")
	}
	if s.Type != "regular" {
		t.Errorf("Type = %q, want regular", s.Type)
	}
	if s.Emoji != "😀" {
		t.Errorf("Emoji = %q", s.Emoji)
	}
	if s.SetName != "MySet" {
		t.Errorf("SetName = %q, want MySet", s.SetName)
	}
	if s.Width != 512 || s.Height != 512 {
		t.Errorf("dimensions = %dx%d, want 512x512", s.Width, s.Height)
	}
	// image/webp mime → static sticker (not animated, not video).
	if s.IsAnimated || s.IsVideo {
		t.Errorf("webp should be static, got animated=%v video=%v", s.IsAnimated, s.IsVideo)
	}
	if s.FileSize != 1024 {
		t.Errorf("FileSize = %d, want 1024", s.FileSize)
	}
	if s.FileID == "" || s.FileUniqueID == "" {
		t.Error("FileID/FileUniqueID should be populated")
	}
	if s.Thumbnail == nil || s.Thumb == nil {
		t.Error("thumbnail should be populated")
	}
}

func TestStickerFromDocument_AnimatedTGS(t *testing.T) {
	doc := &tg.Document{
		ID:       1,
		MimeType: "application/x-tgsticker",
		Attributes: []tg.DocumentAttributeClass{
			&tg.DocumentAttributeSticker{Alt: "👍"},
		},
	}
	s := StickerFromDocument(doc, nil, "")
	if !s.IsAnimated {
		t.Error("TGS mime should set IsAnimated")
	}
}

func TestStickerFromDocument_VideoWebM(t *testing.T) {
	doc := &tg.Document{
		ID:       1,
		MimeType: "video/webm",
		Attributes: []tg.DocumentAttributeClass{
			&tg.DocumentAttributeVideo{W: 512, H: 512},
		},
	}
	s := StickerFromDocument(doc, nil, "")
	if !s.IsVideo {
		t.Error("webm mime should set IsVideo")
	}
	if s.Width != 512 || s.Height != 512 {
		t.Errorf("video dims = %dx%d", s.Width, s.Height)
	}
}

func TestStickerFromDocument_CustomEmoji(t *testing.T) {
	doc := &tg.Document{
		ID: 999,
		Attributes: []tg.DocumentAttributeClass{
			&tg.DocumentAttributeCustomEmoji{Alt: "🎉"},
		},
	}
	s := StickerFromDocument(doc, nil, "EmojiSet")
	if s.Type != "custom_emoji" {
		t.Errorf("Type = %q, want custom_emoji", s.Type)
	}
	if s.CustomEmojiID != "999" {
		t.Errorf("CustomEmojiID = %q, want 999", s.CustomEmojiID)
	}
	// Caller-provided setName wins over attribute.
	if s.SetName != "EmojiSet" {
		t.Errorf("SetName = %q, want EmojiSet", s.SetName)
	}
	// Custom emoji falls back to its Alt emoji.
	if s.Emoji != "🎉" {
		t.Errorf("Emoji = %q", s.Emoji)
	}
}

func TestStickerFromDocument_MaskSticker(t *testing.T) {
	doc := &tg.Document{
		ID: 5,
		Attributes: []tg.DocumentAttributeClass{
			&tg.DocumentAttributeSticker{
				Mask: true,
				MaskCoords: &tg.MaskCoords{N: 1, X: 0.1, Y: 0.2, Zoom: 1.5},
			},
		},
	}
	s := StickerFromDocument(doc, nil, "")
	if s.Type != "mask" {
		t.Errorf("Type = %q, want mask", s.Type)
	}
	if s.MaskPosition == nil {
		t.Fatal("MaskPosition should be set")
	}
	if s.MaskPosition.Point != "eyes" {
		t.Errorf("MaskPosition.Point = %q, want eyes", s.MaskPosition.Point)
	}
}

func TestStickerFromDocument_EmojiFromMap(t *testing.T) {
	// When Alt is empty, fall back to the emoji map.
	doc := &tg.Document{
		ID: 7,
		Attributes: []tg.DocumentAttributeClass{
			&tg.DocumentAttributeSticker{},
		},
	}
	s := StickerFromDocument(doc, map[int64]string{7: "🐶"}, "")
	if s.Emoji != "🐶" {
		t.Errorf("Emoji = %q, want 🐶", s.Emoji)
	}
}

// --- maskCoordsToPosition ---------------------------------------------------

func TestMaskCoordsToPosition(t *testing.T) {
	tests := []struct {
		n       int
		want    string
		wantNil bool
	}{
		{0, "forehead", false},
		{1, "eyes", false},
		{2, "mouth", false},
		{3, "chin", false},
		{4, "", true}, // out of range → nil
	}
	for _, tt := range tests {
		mc := &tg.MaskCoords{N: int32(tt.n), X: 0.5, Y: 0.5, Zoom: 2.0}
		got := maskCoordsToPosition(mc)
		if tt.wantNil {
			if got != nil {
				t.Errorf("N=%d: expected nil, got %+v", tt.n, got)
			}
			continue
		}
		if got == nil {
			t.Errorf("N=%d: expected %s, got nil", tt.n, tt.want)
			continue
		}
		if got.Point != tt.want {
			t.Errorf("N=%d: Point = %q, want %q", tt.n, got.Point, tt.want)
		}
	}
	// nil coords → nil.
	if maskCoordsToPosition(nil) != nil {
		t.Error("nil MaskCoords should return nil")
	}
}

// --- buildEmojiMap ----------------------------------------------------------

func TestBuildEmojiMap(t *testing.T) {
	packs := []*tg.StickerPack{
		{Emoticon: "🎉", Documents: []int64{1, 2}},
		{Emoticon: "👍", Documents: []int64{2, 3}},
	}
	m := buildEmojiMap(packs)
	// First pack wins for doc 2 → 🎉.
	if m[1] != "🎉" || m[2] != "🎉" || m[3] != "👍" {
		t.Errorf("emoji map = %v", m)
	}
	if len(m) != 3 {
		t.Errorf("map size = %d, want 3", len(m))
	}
}

// --- StickerDocSetID --------------------------------------------------------

func TestStickerDocSetID(t *testing.T) {
	doc := &tg.Document{
		Attributes: []tg.DocumentAttributeClass{
			&tg.DocumentAttributeSticker{
				Stickerset: &tg.InputStickerSetID{ID: 42, AccessHash: 1},
			},
		},
	}
	id, ok := StickerDocSetID(doc)
	if !ok || id != 42 {
		t.Errorf("StickerDocSetID = %d/%v, want 42/true", id, ok)
	}
	// Short-name reference → not resolvable by ID.
	doc2 := &tg.Document{
		Attributes: []tg.DocumentAttributeClass{
			&tg.DocumentAttributeSticker{
				Stickerset: &tg.InputStickerSetShortName{ShortName: "x"},
			},
		},
	}
	if _, ok := StickerDocSetID(doc2); ok {
		t.Error("short-name set should return ok=false")
	}
	// Custom emoji attribute also carries a set reference.
	doc3 := &tg.Document{
		Attributes: []tg.DocumentAttributeClass{
			&tg.DocumentAttributeCustomEmoji{
				Stickerset: &tg.InputStickerSetID{ID: 7, AccessHash: 1},
			},
		},
	}
	if id, ok := StickerDocSetID(doc3); !ok || id != 7 {
		t.Errorf("custom emoji StickerDocSetID = %d/%v, want 7/true", id, ok)
	}
	// nil / no attribute.
	if _, ok := StickerDocSetID(nil); ok {
		t.Error("nil should return ok=false")
	}
	if _, ok := StickerDocSetID(&tg.Document{}); ok {
		t.Error("no sticker attribute should return ok=false")
	}
}

// --- setThumbPhotoSize ------------------------------------------------------

func TestSetThumbPhotoSize_NoVersion(t *testing.T) {
	if ps := setThumbPhotoSize(&tg.StickerSet{}); ps != nil {
		t.Error("ThumbVersion==0 should return nil")
	}
	if ps := setThumbPhotoSize(nil); ps != nil {
		t.Error("nil set should return nil")
	}
}

func TestSetThumbPhotoSize_WithVersion(t *testing.T) {
	sset := &tg.StickerSet{
		ID:           100,
		AccessHash:   200,
		ThumbDCID:    2,
		ThumbVersion: 9,
		Thumbs: []tg.PhotoSizeClass{
			&tg.PhotoSize{Type: "m", W: 320, H: 320, Size: 8192},
		},
	}
	ps := setThumbPhotoSize(sset)
	if ps == nil {
		t.Fatal("expected PhotoSize, got nil")
	}
	if ps.FileID == "" || ps.FileUniqueID == "" {
		t.Error("FileID/FileUniqueID should be set")
	}
	if ps.Width != 320 || ps.Height != 320 || ps.FileSize != 8192 {
		t.Errorf("thumb dims = %dx%d size=%d", ps.Width, ps.Height, ps.FileSize)
	}
}

// --- StickersFromDocuments --------------------------------------------------

func TestStickersFromDocuments(t *testing.T) {
	docs := []tg.DocumentClass{
		&tg.Document{ID: 1, Attributes: []tg.DocumentAttributeClass{&tg.DocumentAttributeSticker{Alt: "a"}}},
		&tg.Document{ID: 2, Attributes: []tg.DocumentAttributeClass{&tg.DocumentAttributeSticker{Alt: "b"}}},
		// Non-document entries are skipped.
		&tg.DocumentEmpty{},
	}
	out := StickersFromDocuments(docs, "Set")
	if len(out) != 2 {
		t.Fatalf("len = %d, want 2", len(out))
	}
	if out[0].SetName != "Set" {
		t.Errorf("out[0].SetName = %q", out[0].SetName)
	}
	if out[0].Emoji != "a" {
		t.Errorf("out[0].Emoji = %q", out[0].Emoji)
	}
}

// --- StickerSetFromMessages -------------------------------------------------

func TestStickerSetFromMessages_Nil(t *testing.T) {
	if StickerSetFromMessages(nil) != nil {
		t.Error("nil should return nil")
	}
}

func TestStickerSetFromMessages_Regular(t *testing.T) {
	ss := &tg.MessagesStickerSet{
		Set: &tg.StickerSet{
			ID:           10,
			AccessHash:   20,
			Title:        "Cool Set",
			ShortName:    "coolset",
			ThumbDCID:    2,
			ThumbVersion: 5,
			Thumbs: []tg.PhotoSizeClass{
				&tg.PhotoSize{Type: "m", W: 128, H: 128, Size: 2048},
			},
		},
		Packs: []*tg.StickerPack{
			{Emoticon: "🐶", Documents: []int64{100}},
		},
		Documents: []tg.DocumentClass{
			&tg.Document{
				ID: 100, DCID: 2, AccessHash: 30,
				MimeType: "image/webp",
				Attributes: []tg.DocumentAttributeClass{
					&tg.DocumentAttributeImageSize{W: 512, H: 512},
					&tg.DocumentAttributeSticker{},
				},
			},
		},
	}
	out := StickerSetFromMessages(ss)
	if out == nil {
		t.Fatal("nil output")
	}
	if out.Name != "coolset" || out.Title != "Cool Set" {
		t.Errorf("Name/Title = %q/%q", out.Name, out.Title)
	}
	if out.StickerType != "regular" {
		t.Errorf("StickerType = %q, want regular", out.StickerType)
	}
	if out.Thumbnail == nil {
		t.Error("set thumbnail should be set (versioned thumb)")
	}
	if len(out.Stickers) != 1 {
		t.Fatalf("stickers = %d, want 1", len(out.Stickers))
	}
	if out.Stickers[0].Emoji != "🐶" {
		t.Errorf("sticker emoji = %q, want 🐶", out.Stickers[0].Emoji)
	}
	if out.Stickers[0].SetName != "coolset" {
		t.Errorf("sticker set name = %q", out.Stickers[0].SetName)
	}
}

func TestStickerSetFromMessages_CustomEmojiType(t *testing.T) {
	ss := &tg.MessagesStickerSet{
		Set: &tg.StickerSet{ShortName: "emo", Emojis: true},
		Documents: []tg.DocumentClass{
			&tg.Document{
				ID: 1,
				Attributes: []tg.DocumentAttributeClass{
					&tg.DocumentAttributeCustomEmoji{},
				},
			},
		},
	}
	out := StickerSetFromMessages(ss)
	if out.StickerType != "custom_emoji" {
		t.Errorf("StickerType = %q, want custom_emoji", out.StickerType)
	}
	if len(out.Stickers) != 1 || out.Stickers[0].Type != "custom_emoji" {
		t.Errorf("stickers = %+v", out.Stickers)
	}
}

func TestStickerSetFromMessages_MaskType(t *testing.T) {
	ss := &tg.MessagesStickerSet{
		Set: &tg.StickerSet{ShortName: "masks", Masks: true},
		Documents: []tg.DocumentClass{
			&tg.Document{
				ID: 1,
				Attributes: []tg.DocumentAttributeClass{
					&tg.DocumentAttributeSticker{Mask: true},
				},
			},
		},
	}
	out := StickerSetFromMessages(ss)
	if out.StickerType != "mask" || !out.ContainsMasks {
		t.Errorf("StickerType=%q ContainsMasks=%v", out.StickerType, out.ContainsMasks)
	}
}

func TestStickerSetFromMessages_ThumbDocumentID(t *testing.T) {
	// When ThumbVersion==0 but ThumbDocumentID is set, the thumbnail comes from
	// the Documents array.
	ss := &tg.MessagesStickerSet{
		Set: &tg.StickerSet{
			ShortName:       "doc",
			ThumbDocumentID: 777,
		},
		Documents: []tg.DocumentClass{
			&tg.Document{
				ID: 777, DCID: 4, AccessHash: 5, Size: 999,
				Attributes: []tg.DocumentAttributeClass{
					&tg.DocumentAttributeImageSize{W: 100, H: 100},
				},
			},
		},
	}
	out := StickerSetFromMessages(ss)
	if out.Thumbnail == nil {
		t.Fatal("thumbnail from ThumbDocumentID should be set")
	}
	if out.Thumbnail.Width != 100 || out.Thumbnail.Height != 100 {
		t.Errorf("thumb dims = %dx%d", out.Thumbnail.Width, out.Thumbnail.Height)
	}
}
