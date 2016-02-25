package ircmessage

import (
	"strings"
	"testing"
)

const (
	prefix    = ":nickname!user@example.com"
	raw       = prefix + "PRIVMSG #example :hello there"
	rawTagged = "@test=super;single " + raw
)

func BenchmarkScan(b *testing.B) {
	for n := 0; n < b.N; n++ {
		b.StopTimer()
		scanner := NewScanner(strings.NewReader(raw))
		b.StartTimer()
		scanner.Scan()
		scanner.Message()
	}
}

func BenchmarkScanTagged(b *testing.B) {
	for n := 0; n < b.N; n++ {
		b.StopTimer()
		scanner := NewScanner(strings.NewReader(rawTagged))
		b.StartTimer()
		scanner.Scan()
		scanner.Message()
	}
}

func BenchmarkParsePrefix(b *testing.B) {
	for n := 0; n < b.N; n++ {
		if p := ParsePrefix(prefix); p == nil {
			b.Error("parsed prefix should not be nil")
		}
	}
}
