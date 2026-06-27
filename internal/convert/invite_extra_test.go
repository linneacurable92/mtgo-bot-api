package convert

import (
	"testing"

	"github.com/mtgo-labs/mtgo/tg"
)

func TestChatInviteLinkFromExported_Nil(t *testing.T) {
	if ChatInviteLinkFromExported(nil) != nil {
		t.Error("nil should return nil")
	}
}

func TestChatInviteLinkFromExported_Basic(t *testing.T) {
	e := &tg.ChatInviteExported{
		Link:      "https://t.me/+abc",
		AdminID:   100,
		Permanent: true,
		Revoked:   false,
		Title:     "My Link",
		ExpireDate: 1700000000,
		UsageLimit: 50,
		Requested:  3,
		RequestNeeded: true,
	}
	link := ChatInviteLinkFromExported(e)
	if link == nil {
		t.Fatal("nil link")
	}
	if link.InviteLink != "https://t.me/+abc" {
		t.Errorf("InviteLink = %q", link.InviteLink)
	}
	if link.Creator.ID != 100 {
		t.Errorf("Creator.ID = %d, want 100", link.Creator.ID)
	}
	if !link.IsPrimary {
		t.Error("IsPrimary should be true (Permanent)")
	}
	if link.IsRevoked {
		t.Error("IsRevoked should be false")
	}
	if link.Name != "My Link" {
		t.Errorf("Name = %q", link.Name)
	}
	if link.ExpireDate != 1700000000 {
		t.Errorf("ExpireDate = %d", link.ExpireDate)
	}
	if link.MemberLimit != 50 {
		t.Errorf("MemberLimit = %d, want 50", link.MemberLimit)
	}
	if link.PendingJoinRequestCount != 3 {
		t.Errorf("PendingJoinRequestCount = %d", link.PendingJoinRequestCount)
	}
	if !link.CreatesJoinRequest {
		t.Error("CreatesJoinRequest should be true")
	}
}

func TestChatInviteLinkFromExported_WithSubscription(t *testing.T) {
	e := &tg.ChatInviteExported{
		Link:                "https://t.me/+sub",
		AdminID:             1,
		SubscriptionPricing: &tg.StarsSubscriptionPricing{Period: 30, Amount: 100},
	}
	link := ChatInviteLinkFromExported(e)
	if link.SubscriptionPeriod != 30 || link.SubscriptionPrice != 100 {
		t.Errorf("Subscription = %d/%d, want 30/100", link.SubscriptionPeriod, link.SubscriptionPrice)
	}
}
