package mailsync

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

// maxBodyBytes caps how much of a message body is held in memory for parsing.
const maxBodyBytes = 256 * 1024

// Source reads messages from a mailbox. It is an interface so the sync logic
// can be exercised with a fake in tests.
type Source interface {
	// ListIDs returns message ids matching a Gmail search query (newest first)
	// and whether more results exist beyond max.
	ListIDs(ctx context.Context, query string, max int64) (ids []string, more bool, err error)
	Get(ctx context.Context, id string) (Message, error)
	ProfileEmail(ctx context.Context) (string, error)
}

// NewSourceFunc builds a Source from an OAuth token source.
type NewSourceFunc func(ctx context.Context, ts oauth2.TokenSource) (Source, error)

type googleSource struct{ svc *gmail.Service }

func newGoogleSource(ctx context.Context, ts oauth2.TokenSource) (Source, error) {
	svc, err := gmail.NewService(ctx, option.WithTokenSource(ts))
	if err != nil {
		return nil, fmt.Errorf("create gmail client: %w", err)
	}
	return &googleSource{svc: svc}, nil
}

func (g *googleSource) ProfileEmail(ctx context.Context) (string, error) {
	p, err := g.svc.Users.GetProfile("me").Context(ctx).Do()
	if err != nil {
		return "", err
	}
	return strings.ToLower(p.EmailAddress), nil
}

func (g *googleSource) ListIDs(ctx context.Context, query string, max int64) ([]string, bool, error) {
	resp, err := g.svc.Users.Messages.List("me").Q(query).MaxResults(max).Context(ctx).Do()
	if err != nil {
		return nil, false, err
	}
	ids := make([]string, 0, len(resp.Messages))
	for _, m := range resp.Messages {
		ids = append(ids, m.Id)
	}
	return ids, resp.NextPageToken != "", nil
}

func (g *googleSource) Get(ctx context.Context, id string) (Message, error) {
	m, err := g.svc.Users.Messages.Get("me", id).Format("full").Context(ctx).Do()
	if err != nil {
		return Message{}, err
	}
	out := Message{ID: m.Id, ReceivedAt: time.UnixMilli(m.InternalDate).UTC()}
	if m.Payload != nil {
		for _, h := range m.Payload.Headers {
			switch strings.ToLower(h.Name) {
			case "from":
				out.From = h.Value
			case "subject":
				out.Subject = h.Value
			}
		}
		var sb strings.Builder
		collectBody(m.Payload, &sb)
		out.Body = sb.String()
	}
	// Normalize the From header to a bare address for storage in the review queue.
	if a, err := mail.ParseAddress(out.From); err == nil {
		out.From = a.Address
	}
	return out, nil
}

// collectBody appends decoded text/plain and text/html parts, up to maxBodyBytes.
func collectBody(p *gmail.MessagePart, sb *strings.Builder) {
	if p == nil || sb.Len() >= maxBodyBytes {
		return
	}
	if (p.MimeType == "text/plain" || p.MimeType == "text/html") && p.Body != nil && p.Body.Data != "" {
		data := strings.TrimRight(p.Body.Data, "=")
		if raw, err := base64.RawURLEncoding.DecodeString(data); err == nil {
			sb.Write(raw)
			sb.WriteByte('\n')
		}
	}
	for _, child := range p.Parts {
		collectBody(child, sb)
	}
}
