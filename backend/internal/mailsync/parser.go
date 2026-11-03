// Package mailsync implements the read-only Gmail integration: OAuth
// connection, manual sync, conservative matching of LinkedIn invitation
// acceptance emails, and the unmatched-email review queue.
package mailsync

import (
	"net/mail"
	"regexp"
	"strings"
	"time"
)

// Message is the minimal view of an email the parser needs. Bodies are held in
// memory only while parsing; they are never stored or logged.
type Message struct {
	ID         string
	From       string
	Subject    string
	ReceivedAt time.Time
	Body       string
}

// ParseResult is what a parser extracts from an acceptance email.
type ParseResult struct {
	PersonName string // as written in the email, may be empty
	ProfileURL string // linkedin.com/in/... URL if present, may be empty
}

// Parser recognises one kind of email. Returning ok=false means "not an
// acceptance email" and the message is recorded as IGNORED.
type Parser interface {
	Parse(m Message) (ParseResult, bool)
}

// LinkedInAcceptanceParser is a PROVISIONAL implementation.
//
// It was written without a real sample email, so it only relies on two loose
// signals: the sender is a linkedin.com address and the subject reads
// "<name> accepted your invitation". Everything it cannot confidently recognise
// is ignored, and anything it recognises but cannot match to exactly one contact
// goes to the review queue, so a wrong pattern degrades to "needs manual review",
// not to wrong data. Replace it (the Parser interface stays the same) once a
// sanitized real acceptance email is available.
type LinkedInAcceptanceParser struct{}

var (
	acceptedSubject = regexp.MustCompile(`(?i)^\s*(.{2,100}?)\s+accepted your invitation`)
	profileURLRe    = regexp.MustCompile(`https?://(?:[a-z]{2,3}\.)?linkedin\.com/(?:comm/)?in/[A-Za-z0-9\-_%]+`)
)

func (LinkedInAcceptanceParser) Parse(m Message) (ParseResult, bool) {
	if !isLinkedInSender(m.From) {
		return ParseResult{}, false
	}
	sm := acceptedSubject.FindStringSubmatch(m.Subject)
	if sm == nil {
		return ParseResult{}, false
	}
	res := ParseResult{PersonName: strings.TrimSpace(sm[1])}
	if u := profileURLRe.FindString(m.Body); u != "" {
		res.ProfileURL = strings.Replace(u, "/comm/in/", "/in/", 1)
	}
	return res, true
}

func isLinkedInSender(from string) bool {
	addr, err := mail.ParseAddress(from)
	if err != nil {
		return false
	}
	at := strings.LastIndex(addr.Address, "@")
	if at < 0 {
		return false
	}
	domain := strings.ToLower(addr.Address[at+1:])
	return domain == "linkedin.com" || strings.HasSuffix(domain, ".linkedin.com")
}
