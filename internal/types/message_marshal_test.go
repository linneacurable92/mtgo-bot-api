package types

import (
	"encoding/json"
	"testing"
)

func jsonContains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || indexOfStr(s, substr) >= 0)
}

func indexOfStr(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// TestPaidMediaMarshalJSON covers all switch branches: preview, photo,
// live_photo, video, and the default fallback.
func TestPaidMediaMarshalJSON(t *testing.T) {
	cases := []struct {
		name   string
		media  PaidMedia
		wantHas []string
		wantNot []string
	}{
		{
			name:    "preview with dims",
			media:   PaidMedia{Type: "preview", Width: 100, Height: 50, Duration: 30},
			wantHas: []string{`"type":"preview"`, `"width":100`, `"height":50`, `"duration":30`},
		},
		{
			name:    "preview minimal",
			media:   PaidMedia{Type: "preview"},
			wantHas: []string{`"type":"preview"`},
			wantNot: []string{`"width"`, `"height"`, `"duration"`},
		},
		{
			name:    "photo",
			media:   PaidMedia{Type: "photo", Photo: []PhotoSize{{FileID: "p1", Width: 1, Height: 1}}},
			wantHas: []string{`"type":"photo"`, `"photo"`, `"p1"`},
		},
		{
			name:    "live_photo",
			media:   PaidMedia{Type: "live_photo", LivePhoto: &LivePhoto{}},
			wantHas: []string{`"type":"live_photo"`, `"live_photo"`},
		},
		{
			name:    "video",
			media:   PaidMedia{Type: "video", Video: &Video{FileID: "vid"}},
			wantHas: []string{`"type":"video"`, `"video"`, `"vid"`},
		},
		{
			name:    "other fallback",
			media:   PaidMedia{Type: "other"},
			wantHas: []string{`"type":"other"`},
			wantNot: []string{`"photo"`, `"video"`, `"live_photo"`, `"width"`},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := json.Marshal(tc.media)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			s := string(b)
			for _, want := range tc.wantHas {
				if !jsonContains(s, want) {
					t.Errorf("expected %q in output; got %s", want, s)
				}
			}
			for _, unwanted := range tc.wantNot {
				if jsonContains(s, unwanted) {
					t.Errorf("did not expect %q in output; got %s", unwanted, s)
				}
			}
		})
	}
}

// TestBackgroundTypeMarshalJSON covers all switch branches.
func TestBackgroundTypeMarshalJSON(t *testing.T) {
	cases := []struct {
		name    string
		bg      BackgroundType
		wantHas []string
	}{
		{"wallpaper", BackgroundType{Type: "wallpaper", Document: &Document{FileID: "d"}, DarkThemeDimming: -1, IsBlurred: true, IsMoving: true},
			[]string{`"type":"wallpaper"`, `"dark_theme_dimming":-1`, `"is_blurred":true`, `"is_moving":true`}},
		{"pattern", BackgroundType{Type: "pattern", Document: &Document{FileID: "pd"}, Fill: &BackgroundFill{Type: "solid"}, Intensity: 50, IsInverted: true, IsMoving: true},
			[]string{`"type":"pattern"`, `"intensity":50`, `"is_inverted":true`}},
		{"fill", BackgroundType{Type: "fill", Fill: &BackgroundFill{Type: "solid"}, DarkThemeDimming: 0},
			[]string{`"type":"fill"`, `"dark_theme_dimming"`}},
		{"chat_theme", BackgroundType{Type: "chat_theme", ThemeName: "midnight"},
			[]string{`"type":"chat_theme"`, `"theme_name":"midnight"`}},
		{"unknown", BackgroundType{Type: "weird"},
			[]string{`"type":"weird"`}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := json.Marshal(tc.bg)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			s := string(b)
			for _, want := range tc.wantHas {
				if !jsonContains(s, want) {
					t.Errorf("expected %q; got %s", want, s)
				}
			}
		})
	}
}

// TestBackgroundFillMarshalJSON covers all switch branches.
func TestBackgroundFillMarshalJSON(t *testing.T) {
	cases := []struct {
		name    string
		fill    BackgroundFill
		wantHas []string
	}{
		{"solid", BackgroundFill{Type: "solid", Color: 16777215},
			[]string{`"type":"solid"`, `"color":16777215`}},
		{"gradient", BackgroundFill{Type: "gradient", TopColor: 1, BottomColor: 2, RotationAngle: 90},
			[]string{`"type":"gradient"`, `"top_color":1`, `"bottom_color":2`, `"rotation_angle":90`}},
		{"freeform_gradient", BackgroundFill{Type: "freeform_gradient", Colors: []int{1, 2, 3}},
			[]string{`"type":"freeform_gradient"`, `"colors":[1,2,3]`}},
		{"unknown", BackgroundFill{Type: "mystery"},
			[]string{`"type":"mystery"`}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := json.Marshal(tc.fill)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			s := string(b)
			for _, want := range tc.wantHas {
				if !jsonContains(s, want) {
					t.Errorf("expected %q; got %s", want, s)
				}
			}
		})
	}
}

// TestLocationMarshalJSON_Live covers the live-location branch (LivePeriod set).
func TestLocationMarshalJSON_Live(t *testing.T) {
	loc := Location{Latitude: 12.5, Longitude: -34.0, LivePeriod: 3600, Heading: 90, ProximityAlertRadius: 100, HorizontalAccuracy: 5.0}
	b, err := json.Marshal(loc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	for _, want := range []string{`"live_period":3600`, `"heading":90`, `"proximity_alert_radius":100`, `"latitude":12.5`} {
		if !jsonContains(s, want) {
			t.Errorf("expected %q; got %s", want, s)
		}
	}
}
