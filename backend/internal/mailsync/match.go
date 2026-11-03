package mailsync

import (
	"strings"

	"github.com/google/uuid"

	"referralautomation/internal/contact"
)

// Target is a contact with an outstanding invitation (candidate for a match).
type Target struct {
	ContactID   uuid.UUID
	FullName    string
	LinkedInURL *string
}

// Unmatched reasons stored in unmatched_gmail_emails.reason.
const (
	ReasonNoCandidate = "NO_CANDIDATE"
	ReasonAmbiguous   = "AMBIGUOUS"
)

// matchContact applies deliberately conservative rules and returns a contact
// only when exactly one candidate fits:
//
//  1. If the email carries a profile URL, only an exact (normalized) URL match counts.
//     A URL that matches nothing is never "rescued" by name matching.
//  2. Otherwise the extracted name must equal exactly one candidate's full name
//     (case- and whitespace-insensitive).
func matchContact(res ParseResult, targets []Target) (uuid.UUID, string, bool) {
	if res.ProfileURL != "" {
		want, ok := contact.NormalizeLinkedInURL(res.ProfileURL)
		if !ok {
			return uuid.Nil, ReasonNoCandidate, false
		}
		found := map[uuid.UUID]struct{}{}
		for _, t := range targets {
			if t.LinkedInURL == nil {
				continue
			}
			if got, ok := contact.NormalizeLinkedInURL(*t.LinkedInURL); ok && got == want {
				found[t.ContactID] = struct{}{}
			}
		}
		return single(found)
	}

	name := normalizeName(res.PersonName)
	if name == "" {
		return uuid.Nil, ReasonNoCandidate, false
	}
	found := map[uuid.UUID]struct{}{}
	for _, t := range targets {
		if normalizeName(t.FullName) == name {
			found[t.ContactID] = struct{}{}
		}
	}
	return single(found)
}

func single(found map[uuid.UUID]struct{}) (uuid.UUID, string, bool) {
	switch len(found) {
	case 0:
		return uuid.Nil, ReasonNoCandidate, false
	case 1:
		for id := range found {
			return id, "", true
		}
	}
	return uuid.Nil, ReasonAmbiguous, false
}

func normalizeName(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}
