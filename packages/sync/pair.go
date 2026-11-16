package sync

import (
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"strings"
)

// Fingerprint is the SHA-256 of a certificate's DER bytes, by which a phone pins the desktop's.
func Fingerprint(der []byte) []byte {
	sum := sha256.Sum256(der)
	return sum[:]
}

// ShortFingerprint is the form of fingerprint a person compares by eye, as the desktop shows it beside the code and a phone paired by hand shows it before it trusts the desktop: its first 80 bits in base32, four groups of four, as "MZXW 6YTB OJUW U3DP". "" for a fingerprint shorter than that.
func ShortFingerprint(fingerprint []byte) string {
	if len(fingerprint) < 10 {
		return ""
	}
	s := base32.StdEncoding.EncodeToString(fingerprint[:10])
	return s[0:4] + " " + s[4:8] + " " + s[8:12] + " " + s[12:16]
}

// PairURL is what the desktop's pairing QR code holds: kando://pair?a=<ip:port>[,<ip:port>…]&c=<code>&fp=<fingerprint, base64url>&n=<desktop name>.
type PairURL struct {
	// Addrs are where the desktop listens, each an IP address and a port.
	Addrs []netip.AddrPort
	// Code is the one-time pairing code.
	Code string
	// Fingerprint is the SHA-256 of the desktop's certificate (Fingerprint).
	Fingerprint []byte
	// Name is the desktop's name.
	Name string
}

// The pairing URL's scheme and host.
const (
	PairScheme = "kando"
	PairHost   = "pair"
)

// String is the URL, its query in the order of the protocol: a, c, fp, n.
func (p PairURL) String() string {
	addrs := make([]string, len(p.Addrs))
	for i, a := range p.Addrs {
		addrs[i] = a.String()
	}
	query := "a=" + url.QueryEscape(strings.Join(addrs, ",")) +
		"&c=" + url.QueryEscape(p.Code) +
		"&fp=" + base64.RawURLEncoding.EncodeToString(p.Fingerprint) +
		"&n=" + url.QueryEscape(p.Name)
	return PairScheme + "://" + PairHost + "?" + query
}

// ErrNotPairURL is ParsePairURL's error for text that is not a kando://pair URL.
var ErrNotPairURL = errors.New("not a Kando pairing URL")

// ParsePairURL reads a pairing URL as String writes it. Every address must be an IP address with a port, the code CodeDigits digits and the fingerprint a SHA-256 in base64url; the name may be empty.
func ParsePairURL(s string) (PairURL, error) {
	u, err := url.Parse(strings.TrimSpace(s))
	if err != nil || u.Scheme != PairScheme || u.Host != PairHost {
		return PairURL{}, ErrNotPairURL
	}
	q := u.Query()
	var p PairURL
	for part := range strings.SplitSeq(q.Get("a"), ",") {
		addr, err := netip.ParseAddrPort(strings.TrimSpace(part))
		if err != nil {
			return PairURL{}, fmt.Errorf("%w: address %q: %v", ErrNotPairURL, part, err)
		}
		p.Addrs = append(p.Addrs, addr)
	}
	p.Code = q.Get("c")
	if !ValidCode(p.Code) {
		return PairURL{}, fmt.Errorf("%w: code %q", ErrNotPairURL, p.Code)
	}
	p.Fingerprint, err = base64.RawURLEncoding.DecodeString(q.Get("fp"))
	if err != nil || len(p.Fingerprint) != sha256.Size {
		return PairURL{}, fmt.Errorf("%w: fingerprint %q", ErrNotPairURL, q.Get("fp"))
	}
	p.Name = q.Get("n")
	return p, nil
}

// ValidCode reports whether code has the form of a pairing code: CodeDigits digits.
func ValidCode(code string) bool {
	if len(code) != CodeDigits {
		return false
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
