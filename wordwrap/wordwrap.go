package wordwrap

import (
	"bytes"
	"strings"
	"unicode"

	"github.com/muesli/reflow/ansi"
)

var (
	defaultBreakpoints = []rune{'-'}
	defaultNewline     = []rune{'\n'}
)

func isCJK(r rune) bool {
	return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana,
		unicode.Hangul) ||
		(r >= 0x3000 && r <= 0x303F) ||
		(r >= 0xFF01 && r <= 0xFF60) ||
		(r >= 0xFFE0 && r <= 0xFFE6)
}

type WordWrap struct {
	Limit        int
	Breakpoints  []rune
	Newline      []rune
	KeepNewlines bool

	buf   bytes.Buffer
	space bytes.Buffer
	word  ansi.Buffer

	lineLen int
	ansi    bool
}

func NewWriter(limit int) *WordWrap {
	return &WordWrap{
		Limit:        limit,
		Breakpoints:  defaultBreakpoints,
		Newline:      defaultNewline,
		KeepNewlines: true,
	}
}

func Bytes(b []byte, limit int) []byte {
	f := NewWriter(limit)
	_, _ = f.Write(b)
	_ = f.Close()
	return f.Bytes()
}

func String(s string, limit int) string {
	return string(Bytes([]byte(s), limit))
}

func (w *WordWrap) addSpace() {
	w.lineLen += w.space.Len()
	_, _ = w.buf.Write(w.space.Bytes())
	w.space.Reset()
}

func (w *WordWrap) addWord() {
	if w.word.Len() > 0 {
		w.addSpace()
		w.lineLen += w.word.PrintableRuneWidth()
		_, _ = w.buf.Write(w.word.Bytes())
		w.word.Reset()
	}
}

func (w *WordWrap) addNewLine() {
	_, _ = w.buf.WriteRune('\n')
	w.lineLen = 0
	w.space.Reset()
}

func inGroup(a []rune, c rune) bool {
	for _, v := range a {
		if v == c {
			return true
		}
	}
	return false
}

func (w *WordWrap) currentLineLen() int {
	return w.lineLen + w.space.Len() + w.word.PrintableRuneWidth()
}

func (w *WordWrap) Write(b []byte) (int, error) {
	if w.Limit == 0 {
		return w.buf.Write(b)
	}

	s := string(b)
	if !w.KeepNewlines {
		s = strings.Replace(strings.TrimSpace(s), "\n", " ", -1)
	}

	var prevCJK bool

	for _, c := range s {
		if c == '\x1B' {
			_, _ = w.word.WriteRune(c)
			w.ansi = true
		} else if w.ansi {
			_, _ = w.word.WriteRune(c)
			if (c >= 0x40 && c <= 0x5a) || (c >= 0x61 && c <= 0x7a) {
				w.ansi = false
			}
		} else if inGroup(w.Newline, c) {
			if w.word.Len() == 0 {
				if w.lineLen+w.space.Len() > w.Limit {
					w.lineLen = 0
				} else {
					_, _ = w.buf.Write(w.space.Bytes())
				}
				w.space.Reset()
			}
			w.addWord()
			w.addNewLine()
			prevCJK = false
		} else if unicode.IsSpace(c) {
			w.addWord()
			_, _ = w.space.WriteRune(c)
			prevCJK = false
		} else if inGroup(w.Breakpoints, c) {
			w.addSpace()
			w.addWord()
			_, _ = w.buf.WriteRune(c)
			prevCJK = false
		} else {
			cjk := isCJK(c)

			if cjk != prevCJK && w.word.PrintableRuneWidth() > 0 {
				w.addWord()
			}
			_, _ = w.word.WriteRune(c)
			prevCJK = cjk

			if cjk {
				// CJK: check BEFORE flushing — if this char would exceed
				// the limit, start a new line first so the char goes there.
				if w.currentLineLen() > w.Limit &&
					w.word.PrintableRuneWidth() < w.Limit {
					w.addNewLine()
				}
				w.addWord()
			} else if w.currentLineLen() > w.Limit &&
				w.word.PrintableRuneWidth() < w.Limit {
				w.addNewLine()
			}
		}
	}

	return len(b), nil
}

func (w *WordWrap) Close() error {
	w.addWord()
	return nil
}

func (w *WordWrap) Bytes() []byte {
	return w.buf.Bytes()
}

func (w *WordWrap) String() string {
	return w.buf.String()
}
