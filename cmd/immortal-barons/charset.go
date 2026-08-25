package main

import (
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
