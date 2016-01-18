package ircmessage

import (
	"io"
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
		m, err := s.Next()
		if err != tt.err {
			t.Errorf("%d. expecting error: %v got %v", i, tt.err, err)
		}
		if !reflect.DeepEqual(tt.expected, m) {
			t.Errorf("%d. expecting message: %#v\nbut received: %#v", i, tt.expected, m)
		}
		if _, err = s.Next(); err != io.EOF {
			t.Errorf("expecting error io.EOF got: %v", err)
		}
	}
}
