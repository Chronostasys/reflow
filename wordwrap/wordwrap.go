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

// CJK line-breaking rules (避头尾规则).
var (
	lineStartProhibited = map[rune]bool{
		'，': true, '。': true, '！': true, '？': true,
		'、': true, '：': true, '；': true,
		'）': true, '】': true, '」': true, '』': true, '》': true,
		'〕': true, '〉': true, '．': true,
		',': true, '.': true, '!': true, '?': true,
		')': true, ']': true,
	}
	lineEndProhibited = map[rune]bool{
		'（': true, '【': true, '「': true, '『': true, '《': true,
		'〔': true, '〈': true,
		'(': true, '[': true,
	}
)

// cjkMaxOverhang is the maximum number of extra display columns a CJK line
// may exceed the soft limit while looking for a better break point.
// This prevents infinite accumulation when there's no natural break point.
const cjkMaxOverhang = 20

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

	lineLen      int
	ansi         bool
	pendingBreak bool
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

// flushPendingBreak executes a deferred line break, moving the buffered
// word to a new line.
func (w *WordWrap) flushPendingBreak() {
	if w.pendingBreak {
		w.addNewLine()
		w.pendingBreak = false
	}
}

// Write is used to write more content to the word-wrap buffer.
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
			w.flushPendingBreak()
			_, _ = w.word.WriteRune(c)
			w.ansi = true
		} else if w.ansi {
			_, _ = w.word.WriteRune(c)
			if (c >= 0x40 && c <= 0x5a) || (c >= 0x61 && c <= 0x7a) {
				w.ansi = false
			}
		} else if inGroup(w.Newline, c) {
			w.pendingBreak = false
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
			// Space is a natural word boundary — break here if pending.
			w.flushPendingBreak()
			w.addWord()
			_, _ = w.space.WriteRune(c)
			prevCJK = false
		} else if inGroup(w.Breakpoints, c) {
			w.flushPendingBreak()
			w.addSpace()
			w.addWord()
			_, _ = w.buf.WriteRune(c)
			prevCJK = false
		} else {
			cjk := isCJK(c)

			if cjk != prevCJK && w.word.PrintableRuneWidth() > 0 {
				w.addWord()
			}

			if w.pendingBreak {
				hardLimit := w.Limit + cjkMaxOverhang
				if w.currentLineLen()+runeWidth(c) > hardLimit && !lineStartProhibited[c] {
					// Hard limit exceeded — force the deferred break.
					// The buffered word goes to a new line.
					w.addNewLine()
					w.pendingBreak = false
				}
				// Otherwise: keep deferring (accumulate in word buffer).
			}

			_, _ = w.word.WriteRune(c)
			prevCJK = cjk

			if cjk {
				prohibitBreak := lineEndProhibited[c]

				if !prohibitBreak && w.currentLineLen() > w.Limit &&
					w.word.PrintableRuneWidth() < w.Limit {
					// Soft limit exceeded — defer the break.
					// Word stays in buffer; we'll look for a better break point.
					w.pendingBreak = true
				} else {
					w.addWord()
				}
			} else if w.currentLineLen() > w.Limit &&
				w.word.PrintableRuneWidth() < w.Limit {
				w.addNewLine()
			}
		}
	}

	return len(b), nil
}

// runeWidth returns the display width of a single rune.
func runeWidth(r rune) int {
	if r >= 0x7F {
		return 2
	}
	return 1
}

// Close finishes the word-wrap operation.
func (w *WordWrap) Close() error {
	w.flushPendingBreak()
	w.addWord()
	return nil
}

func (w *WordWrap) Bytes() []byte {
	return w.buf.Bytes()
}

func (w *WordWrap) String() string {
	return w.buf.String()
}
