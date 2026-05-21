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

// CJK line-breaking prohibition rules (避头尾规则).
var (
	lineStartProhibited = map[rune]bool{
		// CJK punctuation that must NOT start a line
		'，': true, '。': true, '！': true, '？': true,
		'、': true, '：': true, '；': true,
		'）': true, '】': true, '」': true, '』': true, '》': true,
		'〕': true, '〉': true, '．': true,
		// ASCII equivalents
		',': true, '.': true, '!': true, '?': true,
		')': true, ']': true,
	}
	lineEndProhibited = map[rune]bool{
		'（': true, '【': true, '「': true, '『': true, '《': true,
		'〔': true, '〈': true,
		'(': true, '[': true,
	}
)

// isCJK returns true for characters that follow CJK line-breaking rules.
func isCJK(r rune) bool {
	return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana,
		unicode.Hangul) ||
		(r >= 0x3000 && r <= 0x303F) || // CJK Symbols and Punctuation
		(r >= 0xFF01 && r <= 0xFF60) || // Halfwidth and Fullwidth Forms (punct)
		(r >= 0xFFE0 && r <= 0xFFE6) // Fullwidth signs
}

// WordWrap contains settings and state for customisable text reflowing.
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
	pendingBreak bool // deferred line break for kinsoku
}

// NewWriter returns a new instance of a word-wrapping writer.
func NewWriter(limit int) *WordWrap {
	return &WordWrap{
		Limit:        limit,
		Breakpoints:  defaultBreakpoints,
		Newline:      defaultNewline,
		KeepNewlines: true,
	}
}

// Bytes is shorthand for declaring a new default WordWrap instance.
func Bytes(b []byte, limit int) []byte {
	f := NewWriter(limit)
	_, _ = f.Write(b)
	_ = f.Close()
	return f.Bytes()
}

// String is shorthand for declaring a new default WordWrap instance.
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

// flushPendingBreak executes a deferred line break if one was pending.
// Called when the next character is NOT line-start-prohibited.
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

			// Flush pending break before processing new content.
			// If this character is line-start-prohibited, defer the break.
			if w.pendingBreak {
				if lineStartProhibited[c] {
					// Character must NOT start a line — absorb into current line
					w.pendingBreak = false
				} else {
					// Safe to break here
					w.addNewLine()
					w.pendingBreak = false
				}
			}

			if cjk != prevCJK && w.word.PrintableRuneWidth() > 0 {
				w.addWord()
			}
			_, _ = w.word.WriteRune(c)
			prevCJK = cjk

			if cjk {
				prohibitBreak := lineEndProhibited[c]

				if !prohibitBreak && w.currentLineLen() > w.Limit &&
					w.word.PrintableRuneWidth() < w.Limit {
					w.pendingBreak = true
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

// Close will finish the word-wrap operation.
func (w *WordWrap) Close() error {
	w.flushPendingBreak()
	w.addWord()
	return nil
}

// Bytes returns the word-wrapped result as a byte slice.
func (w *WordWrap) Bytes() []byte {
	return w.buf.Bytes()
}

// String returns the word-wrapped result as a string.
func (w *WordWrap) String() string {
	return w.buf.String()
}
