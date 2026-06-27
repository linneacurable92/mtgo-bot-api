package convert

import (
	"testing"

	"github.com/mtgo-labs/mtgo/tg"
)

func TestVideoNote_Nil(t *testing.T) {
	if VideoNote(nil) != nil {
		t.Error("nil document should return nil")
	}
}

func TestVideoNote_Basic(t *testing.T) {
	doc := &tg.Document{
		ID: 1, DCID: 2, AccessHash: 3, Size: 5000,
		Attributes: []tg.DocumentAttributeClass{
			&tg.DocumentAttributeVideo{Duration: 12, W: 360, H: 360, RoundMessage: true},
		},
		Thumbs: []tg.PhotoSizeClass{
			&tg.PhotoSize{Type: "s", W: 128, H: 128, Size: 1024},
		},
	}
	vn := VideoNote(doc)
	if vn == nil {
		t.Fatal("nil VideoNote")
	}
	if vn.Duration != 12 {
		t.Errorf("Duration = %d, want 12", vn.Duration)
	}
	// Round video uses W as diameter.
	if vn.Length != 360 {
		t.Errorf("Length = %d, want 360", vn.Length)
	}
	if vn.FileSize != 5000 {
		t.Errorf("FileSize = %d, want 5000", vn.FileSize)
	}
	if vn.FileID == "" || vn.FileUniqueID == "" {
		t.Error("FileID/FileUniqueID should be set")
	}
	if vn.Thumbnail == nil || vn.Thumb == nil {
		t.Error("thumbnail should be set")
	}
}

func TestVideoNote_NoVideoAttr(t *testing.T) {
	// Without DocumentAttributeVideo, duration/length stay zero but file ids set.
	doc := &tg.Document{ID: 1, DCID: 2, AccessHash: 3, Size: 100}
	vn := VideoNote(doc)
	if vn.Duration != 0 || vn.Length != 0 {
		t.Errorf("expected zero duration/length, got %d/%d", vn.Duration, vn.Length)
	}
}

// --- convertPaidMedia -------------------------------------------------------

func TestConvertPaidMedia_Preview(t *testing.T) {
	items := []tg.MessageExtendedMediaClass{
		&tg.MessageExtendedMediaPreview{W: 100, H: 200, VideoDuration: 5},
	}
	info := convertPaidMedia(items, 50)
	if info == nil {
		t.Fatal("nil info")
	}
	if info.StarCount != 50 {
		t.Errorf("StarCount = %d, want 50", info.StarCount)
	}
	if len(info.PaidMedia) != 1 {
		t.Fatalf("PaidMedia len = %d, want 1", len(info.PaidMedia))
	}
	pm := info.PaidMedia[0]
	if pm.Type != "preview" || pm.Width != 100 || pm.Height != 200 || pm.Duration != 5 {
		t.Errorf("preview = %+v", pm)
	}
}

func TestConvertPaidMedia_PhotoAndOther(t *testing.T) {
	items := []tg.MessageExtendedMediaClass{
		&tg.MessageExtendedMedia{
			Media: &tg.MessageMediaPhoto{Photo: &tg.Photo{ID: 1, AccessHash: 2}},
		},
		&tg.MessageExtendedMedia{
			Media: &tg.MessageMediaPhoto{}, // non-*tg.Photo inner → "other"
		},
	}
	info := convertPaidMedia(items, 10)
	if len(info.PaidMedia) != 2 {
		t.Fatalf("len = %d, want 2", len(info.PaidMedia))
	}
	if info.PaidMedia[0].Type != "photo" {
		t.Errorf("first type = %q, want photo", info.PaidMedia[0].Type)
	}
	if info.PaidMedia[1].Type != "other" {
		t.Errorf("second type = %q, want other", info.PaidMedia[1].Type)
	}
}

func TestConvertPaidMedia_Empty(t *testing.T) {
	info := convertPaidMedia(nil, 0)
	if info == nil || len(info.PaidMedia) != 0 {
		t.Errorf("empty info = %+v", info)
	}
}

// --- paidMediaFromMedia -----------------------------------------------------

func TestPaidMediaFromMedia_VideoDocument(t *testing.T) {
	doc := &tg.Document{
		ID: 1, DCID: 2, AccessHash: 3,
		Attributes: []tg.DocumentAttributeClass{
			&tg.DocumentAttributeVideo{Duration: 5, W: 128, H: 128},
		},
	}
	// Mark it as a video file type via the animation/generic path: DocumentFileType
	// keys off DocumentAttributeVideo without DocumentAttributeAnimated → TypeVideo.
	pm := paidMediaFromMedia(&tg.MessageMediaDocument{Document: doc})
	if pm.Type != "video" {
		t.Errorf("type = %q, want video", pm.Type)
	}
	if pm.Video == nil {
		t.Error("Video should be populated")
	}
}

func TestPaidMediaFromMedia_NonVideoDocument(t *testing.T) {
	// A document with no video attribute → TypeDocument → "other".
	doc := &tg.Document{ID: 1, DCID: 2, AccessHash: 3}
	pm := paidMediaFromMedia(&tg.MessageMediaDocument{Document: doc})
	if pm.Type != "other" {
		t.Errorf("type = %q, want other", pm.Type)
	}
}

func TestPaidMediaFromMedia_UnknownMedia(t *testing.T) {
	pm := paidMediaFromMedia(&tg.MessageMediaEmpty{})
	if pm.Type != "other" {
		t.Errorf("type = %q, want other", pm.Type)
	}
}

