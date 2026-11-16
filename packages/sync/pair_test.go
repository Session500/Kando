package sync

import (
	"bytes"
	"errors"
	"net/netip"
	"regexp"
	"strings"
	"testing"
)

func TestPairURLReadsBackWhatItWrites(t *testing.T) {
	fp := Fingerprint([]byte("certificate"))
	p := PairURL{
		Addrs:       []netip.AddrPort{netip.MustParseAddrPort("192.168.1.20:52817"), netip.MustParseAddrPort("10.0.0.5:52817")},
		Code:        "042917",
		Fingerprint: fp,
		Name:        "Evan's MacBook Pro & co",
	}
	s := p.String()
	if !strings.HasPrefix(s, "kando://pair?a=192.168.1.20%3A52817%2C10.0.0.5%3A52817&c=042917&fp=") || !strings.Contains(s, "&n=Evan%27s+MacBook+Pro+%26+co") {
		t.Fatalf("pairing URL %q", s)
	}
	got, err := ParsePairURL(s)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Addrs) != 2 || got.Addrs[0] != p.Addrs[0] || got.Addrs[1] != p.Addrs[1] || got.Code != p.Code || !bytes.Equal(got.Fingerprint, fp) || got.Name != p.Name {
		t.Fatalf("read back %+v, want %+v", got, p)
	}
}

func TestParsePairURLRefusesWhatIsNotOne(t *testing.T) {
	good := PairURL{Addrs: []netip.AddrPort{netip.MustParseAddrPort("192.168.1.20:52817")}, Code: "123456", Fingerprint: Fingerprint(nil), Name: "Mac"}.String()
	for name, s := range map[string]string{
		"another scheme": strings.Replace(good, "kando://", "https://", 1),
		"another host":   strings.Replace(good, "://pair", "://open", 1),
		"no address":     strings.Replace(good, "a=192.168.1.20%3A52817", "a=", 1),
		"a host name":    strings.Replace(good, "192.168.1.20", "mac.local", 1),
		"no port":        strings.Replace(good, "%3A52817", "", 1),
		"a short code":   strings.Replace(good, "c=123456", "c=12345", 1),
		"letters":        strings.Replace(good, "c=123456", "c=12345a", 1),
		"a short pin":    PairURL{Addrs: []netip.AddrPort{netip.MustParseAddrPort("192.168.1.20:52817")}, Code: "123456", Fingerprint: []byte{1, 2, 3}}.String(),
	} {
		if _, err := ParsePairURL(s); !errors.Is(err, ErrNotPairURL) {
			t.Errorf("%s: %q read with error %v", name, s, err)
		}
	}
}

func TestShortFingerprintIsFourGroupsOfFour(t *testing.T) {
	got := ShortFingerprint(Fingerprint([]byte("certificate")))
	if !regexp.MustCompile(`^[A-Z2-7]{4} [A-Z2-7]{4} [A-Z2-7]{4} [A-Z2-7]{4}$`).MatchString(got) {
		t.Fatalf("short form %q", got)
	}
	if other := ShortFingerprint(Fingerprint([]byte("another"))); other == got {
		t.Fatalf("two certificates share the short form %q", got)
	}
	if ShortFingerprint([]byte{1, 2}) != "" {
		t.Fatal("a fingerprint too short has a short form")
	}
}
