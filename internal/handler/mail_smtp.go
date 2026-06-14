package handler

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"strings"

	gomail "github.com/emersion/go-message/mail"
	"github.com/emersion/go-smtp"

	"kanban/internal/model"
)

// Inbound SMTP. Optional: started only when an address is configured (main passes
// -smtp-addr). Received messages are parsed and filed into local users' inboxes;
// the local-part of each RCPT address is matched to a username. This keeps the
// app's default zero-listener footprint unless mail receiving is explicitly on.

const smtpMaxMessageBytes = 25 * 1024 * 1024

type smtpBackend struct{ h *Handler }

func (b *smtpBackend) NewSession(c *smtp.Conn) (smtp.Session, error) {
	ip := ""
	if c != nil && c.Conn() != nil {
		ip = c.Conn().RemoteAddr().String()
	}
	return &smtpSession{h: b.h, sourceIP: ip}, nil
}

type smtpSession struct {
	h        *Handler
	from     string
	rcpts    []string
	sourceIP string
}

func (s *smtpSession) Reset()        { s.from = ""; s.rcpts = nil }
func (s *smtpSession) Logout() error { return nil }

func (s *smtpSession) Mail(from string, _ *smtp.MailOptions) error { s.from = from; return nil }
func (s *smtpSession) Rcpt(to string, _ *smtp.RcptOptions) error {
	s.rcpts = append(s.rcpts, to)
	return nil
}

func (s *smtpSession) Data(r io.Reader) error {
	return s.h.ingestInbound(s.from, s.rcpts, r, s.sourceIP)
}

// StartSMTPServer launches an inbound SMTP listener on addr (e.g. ":25").
func (h *Handler) StartSMTPServer(addr string) {
	srv := smtp.NewServer(&smtpBackend{h: h})
	srv.Addr = addr
	srv.Domain = "localhost"
	srv.AllowInsecureAuth = true
	srv.MaxMessageBytes = smtpMaxMessageBytes
	log.Printf("inbound SMTP listening on %s", addr)
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Printf("smtp server: %v", err)
		}
	}()
}

// localPart returns the username portion of an email address.
func localPart(addr string) string {
	addr = strings.TrimSpace(addr)
	if i := strings.LastIndex(addr, "@"); i >= 0 {
		addr = addr[:i]
	}
	return strings.Trim(addr, "<> ")
}

// ingestInbound parses an RFC 5322 message and delivers it to local recipients.
func (h *Handler) ingestInbound(from string, rcpts []string, r io.Reader, sourceIP string) error {
	// Resolve recipients to local users by the address local-part.
	var toIDs []int64
	seen := map[int64]bool{}
	for _, rc := range rcpts {
		u := h.resolveLocalRecipient(localPart(rc))
		if u == nil || seen[u.ID] {
			continue
		}
		seen[u.ID] = true
		toIDs = append(toIDs, u.ID)
	}
	if len(toIDs) == 0 {
		h.store.LogMailAudit(model.MailAudit{Event: "inbound_reject", SourceIP: sourceIP, MailFrom: from,
			Rcpt: strings.Join(rcpts, ", "), Detail: "no local recipients"})
		return fmt.Errorf("no local recipients")
	}

	raw, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read message: %w", err)
	}
	mr, err := gomail.CreateReader(bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("parse message: %w", err)
	}
	subject, _ := mr.Header.Subject()
	fromName, fromAddr := from, from
	if addrs, _ := mr.Header.AddressList("From"); len(addrs) > 0 {
		fromAddr = addrs[0].Address
		if addrs[0].Name != "" {
			fromName = addrs[0].Name
		} else {
			fromName = addrs[0].Address
		}
	}

	var bodyText, bodyHTML string
	var atts []model.MailAttachment
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read part: %w", err)
		}
		switch hdr := p.Header.(type) {
		case *gomail.InlineHeader:
			ct, _, _ := hdr.ContentType()
			data, _ := io.ReadAll(p.Body)
			if strings.EqualFold(ct, "text/html") {
				bodyHTML = sanitizeHTML(string(data))
			} else if bodyText == "" {
				bodyText = string(data)
			}
		case *gomail.AttachmentHeader:
			filename, _ := hdr.Filename()
			data, _ := io.ReadAll(p.Body)
			if len(data) == 0 {
				continue
			}
			ct, _, _ := hdr.ContentType()
			fid, err := h.store.SaveFile(filename, data, ct)
			if err == nil {
				atts = append(atts, model.MailAttachment{FileID: fid, Filename: filename})
			}
		}
	}

	// Antispam scoring: high scores land in Junk.
	attNames := make([]string, len(atts))
	for i, a := range atts {
		attNames[i] = a.Filename
	}
	score, reasons := scoreSpam(SpamSignals{
		Subject: subject, Body: bodyText, Recipients: len(rcpts),
		Attachments: attNames, DKIM: verifyDKIM(raw),
	})
	folder := "inbox"
	if score >= spamThreshold {
		folder = "junk"
	}

	toAddrs := strings.Join(rcpts, ", ")
	id, err := h.store.DeliverInbound(toIDs, fromName, fromAddr, toAddrs, subject, bodyText, bodyHTML, atts, folder, score)
	if err != nil {
		return err
	}
	h.store.LogMailAudit(model.MailAudit{Event: "inbound_accept", SourceIP: sourceIP, MailFrom: fromAddr,
		Rcpt: toAddrs, Subject: subject, Score: score, Detail: fmt.Sprintf("%d recipient(s)", len(toIDs))})
	if folder == "junk" {
		h.store.LogMailAudit(model.MailAudit{Event: "spam", SourceIP: sourceIP, MailFrom: fromAddr,
			Rcpt: toAddrs, Subject: subject, Score: score, Detail: strings.Join(reasons, ", ")})
	}
	// Notify recipients (live + in-app), mirroring internal delivery.
	subj := subject
	if subj == "" {
		subj = "(без темы)"
	}
	for _, uid := range toIDs {
		h.store.CreateNotification(uid, "mail", fmt.Sprintf("Новое письмо от %s: %s", fromName, subj), nil)
	}
	if h.sse != nil {
		h.sse.sendTo(toIDs, map[string]string{"type": "mail"})
	}
	_ = id
	return nil
}
