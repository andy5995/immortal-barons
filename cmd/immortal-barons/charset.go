package main

import (
	"strings"

	"github.com/andy5995/immortal-barons/internal/session"
)

// charset is what the caller's terminal reads. CP437 is the BBS tradition,
// UTF-8 the modern default, and ASCII the fallback for a terminal that is
// neither and would render either as mojibake.
type charset int

// wantCharset resolves the output charset. An explicit -utf8/-cp437/-ascii
// always wins (in that order when more than one is given); otherwise a local
// session follows the process locale, and a door assumes CP437 (its locale
// reflects the BBS server, not the caller's terminal).
func wantCharset(forceUTF8, forceCP437, forceASCII, local bool) charset {
	switch {
	case forceUTF8:
		return charsetUTF8
	case forceCP437:
		return charsetCP437
	case forceASCII:
		return charsetASCII
	case local && session.LocaleIsUTF8():
		return charsetUTF8
	default:
		return charsetCP437
	}
}

// encodeFor wraps s in the writer for cs. UTF-8 needs none: it is what the
// engine already emits.
func encodeFor(s session.Session, cs charset) session.Session {
	switch cs {
	case charsetCP437:
		return session.NewCP437Writer(s)
	case charsetASCII:
		return session.NewASCIIWriter(s)
	default:
		return s
	}
}

// charsetNamed maps an IANA character-set name from a drop file onto what the
// door can actually send. Matching is case-insensitive, as the BBSDEV.DRP spec
// requires, and covers the registry's aliases because a board is free to write
// any of them.
//
// A name the door does not know falls to ASCII rather than to the CP437
// default. That is the deliberate direction: a Latin-1 or CP866 terminal sent
// CP437 renders the box rules and blocks as garbage, while ASCII look-alikes
// are readable on any of them. ok is false only for an empty name, which is
// every format except BBSDEV.DRP.
func charsetNamed(name string) (charset, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "":
		return 0, false
	case "utf-8", "utf8", "csutf8":
		return charsetUTF8, true
	case "ibm437", "cp437", "437", "cspc8codepage437":
		return charsetCP437, true
	case "us-ascii", "ascii", "ansi_x3.4-1968", "ansi_x3.4-1986", "iso646-us",
		"iso_646.irv:1991", "iso-ir-6", "ibm367", "cp367", "us", "csascii":
		return charsetASCII, true
	}
	return charsetASCII, true
}
