package ircmessage

import (
	"reflect"
	"strings"
	"testing"
)

var scannerTests = []struct {
	in       string
	expected Message
	err      error
}{
	{
		"FOO",
		Message{Command: "FOO"},
		nil,
	},
	{
		":test FOO",
		Message{Prefix: "test", Command: "FOO"},
		nil,
	},
	{
		":test FOO     ",
		Message{Prefix: "test", Command: "FOO"},
		nil,
	},
	{
		":test!me@test.ing PRIVMSG #Test :This is a test",
		Message{
			Prefix:  "test!me@test.ing",
			Command: "PRIVMSG",
			Params:  []string{"#Test", "This is a test"},
		},
		nil,
	},
	{
		"PRIVMSG #Test :This is a test",
		Message{
			Command: "PRIVMSG",
			Params:  []string{"#Test", "This is a test"},
		},
		nil,
	},
	{
		":test PRIVMSG foo :A string  with spaces   ",
		Message{
			Prefix:  "test",
			Command: "PRIVMSG",
			Params:  []string{"foo", "A string  with spaces   "},
		},
		nil,
	},
	{
		":test    PRIVMSG   foo    :bar",
		Message{
			Prefix:  "test",
			Command: "PRIVMSG",
			Params:  []string{"foo", "bar"},
		},
		nil,
	},
	{
		":test FOO bar baz quux",
		Message{
			Prefix:  "test",
			Command: "FOO",
			Params:  []string{"bar", "baz", "quux"},
		},
		nil,
	},
	{
		"FOO bar baz quux",
		Message{
			Command: "FOO",
			Params:  []string{"bar", "baz", "quux"},
		},
		nil,
	},
	{
		"FOO   bar    baz  quux",
		Message{
			Command: "FOO",
			Params:  []string{"bar", "baz", "quux"},
		},
		nil,
	},
	{
		"FOO bar baz quux :This is a test",
		Message{
			Command: "FOO",
			Params:  []string{"bar", "baz", "quux", "This is a test"},
		},
		nil,
	},
	{
		":test PRIVMSG #fo:oo :This is a test",
		Message{
			Prefix:  "test",
			Command: "PRIVMSG",
			Params:  []string{"#fo:oo", "This is a test"},
		},
		nil,
	},
	{
		"@test=super;single :test!me@test.ing FOO bar baz quux :This is a test",
		Message{
			Tags: map[string]string{
				"test":   "super",
				"single": "",
			},
			Prefix:  "test!me@test.ing",
			Command: "FOO",
			Params:  []string{"bar", "baz", "quux", "This is a test"},
		},
		nil,
	},
}

func TestScanner(t *testing.T) {
	const eol = "\r\n"
	var s *Scanner
	for i, tt := range scannerTests {
		// Didn't want to repeat the input string in the test
		// suite, so setting it here instead.
		tt.expected.Raw = tt.in + eol
		s = NewScanner(strings.NewReader(tt.in + eol))
		for s.Scan() {
			m := s.Message()
			if !reflect.DeepEqual(tt.expected, m) {
				t.Errorf("%d. expecting message: %#v\nbut received: %#v", i, tt.expected, m)
			}
		}
		if err := s.Err(); err != tt.err {
			t.Errorf("expecting error %v got: %v", tt.err, err)
		}
	}
}

var prefixTests = []struct {
	in       string
	expected *Prefix
}{
	{"nick", &Prefix{Nickname: "nick"}},
	{"nick!user", &Prefix{Nickname: "nick", User: "user"}},
	{"se.rv.er", &Prefix{IsServer: true, Host: "se.rv.er"}},
	{"nick!us.er@host", &Prefix{Nickname: "nick", User: "us.er", Host: "host"}},
	{"nick!", &Prefix{Nickname: "nick"}},
	{"nick@", &Prefix{Nickname: "nick"}},
	{"nick!user!resu@host", &Prefix{Nickname: "nick", User: "user!resu", Host: "host"}},
	{"nick@kcin!user@host", &Prefix{Nickname: "nick@kcin", User: "user", Host: "host"}},
	{"nick!user@host!resu", &Prefix{Nickname: "nick", User: "user", Host: "host!resu"}},
	{"nick!user@host@tsoh", &Prefix{Nickname: "nick", User: "user", Host: "host@tsoh"}},
	{"ni.ck!user@host", &Prefix{Nickname: "ni.ck", User: "user", Host: "host"}},
	{"", nil},
	{"@host", nil},
	{"!user@host", nil},
	{"!@host", nil},
	{"!user@", nil},
}

func TestParsePrefix(t *testing.T) {
	for i, tt := range prefixTests {
		p := ParsePrefix(tt.in)
		if p == nil && tt.expected != nil {
			t.Fatalf("%d. expecting %q, got nil", i, tt.expected)
		}
		if !reflect.DeepEqual(p, tt.expected) {
			t.Errorf("%d. expecting prefix: %v, got %v", i, *tt.expected, *p)
		}
	}
}
