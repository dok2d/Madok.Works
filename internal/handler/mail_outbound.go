package handler

import (
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"log"
	"net"
	mailpkg "net/mail"
	"net/smtp"
	"os"
	"strings"
	"time"

	"github.com/emersion/go-msgauth/dkim"
)

// Outbound mail (S3.4/S3.5). Self-contained and unit-tested for the parts that
// don't need the network (RFC 5322 building + DKIM signing). Actual delivery to
// remote MX hosts is gated behind configuration and only runs when an outbound
// domain is set — it cannot be exercised in a sandbox without public networking.
//
// Config (env):
//   KANBAN_MAIL_DOMAIN   — sending domain (enables outbound when set)
//   KANBAN_DKIM_SELECTOR — DKIM selector (default "default")
//   KANBAN_DKIM_KEY      — path to a PEM RSA private key for DKIM signing

// loadMailOutboundConfig (re)loads outbound config: app_settings first, env as
// fallback. Safe to call again after admin changes settings.
func (h *Handler) loadMailOutboundConfig() {
	h.outboundDomain, h.dkimSelector, h.dkimKey = "", "", nil
	domain := strings.TrimSpace(h.store.GetSetting("mail_domain"))
	if domain == "" {
		domain = strings.TrimSpace(os.Getenv("KANBAN_MAIL_DOMAIN"))
	}
	if domain == "" {
		return
	}
	h.outboundDomain = domain
	sel := strings.TrimSpace(h.store.GetSetting("mail_dkim_selector"))
	if sel == "" {
		sel = strings.TrimSpace(os.Getenv("KANBAN_DKIM_SELECTOR"))
	}
	if sel == "" {
		sel = "default"
	}
	h.dkimSelector = sel
	if pemKey := strings.TrimSpace(h.store.GetSetting("mail_dkim_key")); pemKey != "" {
		if key, err := parseRSAKeyPEM([]byte(pemKey)); err != nil {
			log.Printf("mail: DKIM key (settings) load failed: %v", err)
		} else {
			h.dkimKey = key
		}
	} else if path := strings.TrimSpace(os.Getenv("KANBAN_DKIM_KEY")); path != "" {
		if key, err := loadRSAKey(path); err != nil {
			log.Printf("mail: DKIM key load failed: %v", err)
		} else {
			h.dkimKey = key
		}
	}
	log.Printf("outbound mail enabled for domain %s (dkim=%v)", h.outboundDomain, h.dkimKey != nil)
}

func loadRSAKey(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseRSAKeyPEM(data)
}

func parseRSAKeyPEM(data []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM block")
	}
	if k, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return k, nil
	}
	k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	rk, ok := k.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA key")
	}
	return rk, nil
}

func (h *Handler) outboundEnabled() bool { return h.outboundDomain != "" }

// dkimPublicTXT returns the DNS TXT record value to publish for the DKIM key, or "".
func (h *Handler) dkimPublicTXT() string {
	if h.dkimKey == nil {
		return ""
	}
	der, err := x509.MarshalPKIXPublicKey(&h.dkimKey.PublicKey)
	if err != nil {
		return ""
	}
	return "v=DKIM1; k=rsa; p=" + base64.StdEncoding.EncodeToString(der)
}

// hdrClean strips CR/LF and other control chars from a header value to prevent
// SMTP header/CRLF injection. Header content is always single-line.
func hdrClean(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\r' || r == '\n' || (r < 0x20 && r != '\t') {
			return ' '
		}
		return r
	}, s)
}

// validEmail reports a syntactically valid, injection-free email address.
func validEmail(addr string) bool {
	addr = strings.TrimSpace(addr)
	if addr == "" || strings.ContainsAny(addr, "\r\n") || len(addr) > 320 {
		return false
	}
	a, err := mailpkg.ParseAddress(addr)
	return err == nil && a.Address == addr
}

// buildRFC822 assembles a minimal RFC 5322 text/plain message. All header values
// are run through hdrClean so user input cannot inject extra headers.
func buildRFC822(fromAddr, fromName string, to []string, subject, body, date string) []byte {
	fromAddr, fromName, subject, date = hdrClean(fromAddr), hdrClean(fromName), hdrClean(subject), hdrClean(date)
	cleanTo := make([]string, 0, len(to))
	for _, a := range to {
		cleanTo = append(cleanTo, hdrClean(a))
	}
	var b strings.Builder
	if fromName != "" {
		fmt.Fprintf(&b, "From: %s <%s>\r\n", fromName, fromAddr)
	} else {
		fmt.Fprintf(&b, "From: %s\r\n", fromAddr)
	}
	fmt.Fprintf(&b, "To: %s\r\n", strings.Join(cleanTo, ", "))
	fmt.Fprintf(&b, "Subject: %s\r\n", subject)
	fmt.Fprintf(&b, "Date: %s\r\n", date)
	fmt.Fprintf(&b, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&b, "Content-Type: text/plain; charset=utf-8\r\n")
	b.WriteString("\r\n")
	b.WriteString(strings.ReplaceAll(body, "\n", "\r\n"))
	return []byte(b.String())
}

// dkimSign returns the message with a DKIM-Signature header prepended. If no key
// is configured it returns the input unchanged.
func (h *Handler) dkimSign(raw []byte) ([]byte, error) {
	if h.dkimKey == nil {
		return raw, nil
	}
	var out bytes.Buffer
	opts := &dkim.SignOptions{
		Domain:                 h.outboundDomain,
		Selector:               h.dkimSelector,
		Signer:                 h.dkimKey,
		Hash:                   crypto.SHA256,
		HeaderCanonicalization: dkim.CanonicalizationRelaxed,
		BodyCanonicalization:   dkim.CanonicalizationRelaxed,
		HeaderKeys:             []string{"From", "To", "Subject", "Date", "MIME-Version", "Content-Type"},
	}
	if err := dkim.Sign(&out, bytes.NewReader(raw), opts); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// sendExternal builds, signs and delivers a message to external recipients via
// their MX hosts. No-op unless outbound is enabled.
func (h *Handler) sendExternal(fromName string, to []string, subject, body string) {
	if !h.outboundEnabled() {
		return
	}
	// Drop malformed/injection-bearing recipient addresses before building headers.
	valid := to[:0]
	for _, a := range to {
		if validEmail(a) {
			valid = append(valid, a)
		}
	}
	to = valid
	if len(to) == 0 {
		return
	}
	local := fromName
	if u, err := h.store.GetUserByUsername(fromName); err == nil && u != nil {
		if p := h.store.PrimaryAlias(u.ID); p != "" {
			local = p
		}
	}
	fromAddr := local + "@" + h.outboundDomain
	raw := buildRFC822(fromAddr, fromName, to, subject, body, time.Now().UTC().Format(time.RFC1123Z))
	signed, err := h.dkimSign(raw)
	if err != nil {
		log.Printf("mail: DKIM sign failed: %v", err)
		signed = raw
	}
	byDomain := map[string][]string{}
	for _, addr := range to {
		at := strings.LastIndex(addr, "@")
		if at < 0 {
			continue
		}
		byDomain[addr[at+1:]] = append(byDomain[addr[at+1:]], addr)
	}
	for domain, rcpts := range byDomain {
		if err := deliverToMX(domain, fromAddr, rcpts, signed); err != nil {
			log.Printf("mail: delivery to %s failed: %v", domain, err)
		}
	}
}

// deliverToMX looks up the domain's mail exchanger and sends the message.
func deliverToMX(domain, from string, to []string, msg []byte) error {
	mxs, err := net.LookupMX(domain)
	if err != nil || len(mxs) == 0 {
		return fmt.Errorf("no MX for %s: %v", domain, err)
	}
	host := strings.TrimSuffix(mxs[0].Host, ".")
	// SSRF guard: never connect to an MX that resolves into private/loopback space.
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return fmt.Errorf("cannot resolve MX %s: %v", host, err)
	}
	for _, ip := range ips {
		if isInternalIP(ip) {
			return fmt.Errorf("refusing MX %s: resolves to internal address %s", host, ip)
		}
	}
	return smtp.SendMail(host+":25", nil, from, to, msg)
}

// isInternalIP reports whether ip is loopback, private, link-local or unspecified
// — addresses an outbound mail relay must never be tricked into contacting.
func isInternalIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}
