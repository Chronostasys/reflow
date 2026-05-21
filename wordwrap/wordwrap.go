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

	// CJK kinsoku state: when the soft limit is exceeded, we defer the line
	// break to look for a better position. Instead of a simple boolean, we
	// track the deferred word separately so the main word buffer can be
	// flushed normally.
	deferred      bytes.Buffer // chars deferred during kinsoku lookahead
	deferredWidth int          // display width of deferred chars
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

// flushDeferred moves deferred chars to the current line (absorbs them).
func (w *WordWrap) flushDeferred() {
	if w.deferred.Len() > 0 {
		w.lineLen += w.deferredWidth
		_, _ = w.buf.Write(w.deferred.Bytes())
		w.deferred.Reset()
		w.deferredWidth = 0
	}
}

// breakDeferred starts a new line and puts deferred chars on it.
func (w *WordWrap) breakDeferred() {
	w.addNewLine()
	w.flushDeferred()
}

// hasDeferred returns true if we're in kinsoku lookahead mode.
func (w *WordWrap) hasDeferred() bool {
	return w.deferred.Len() > 0
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
			if w.hasDeferred() {
				// ANSI during deferred — break now, flush deferred to new line
				w.breakDeferred()
			}
			_, _ = w.word.WriteRune(c)
			w.ansi = true
		} else if w.ansi {
			_, _ = w.word.WriteRune(c)
			if (c >= 0x40 && c <= 0x5a) || (c >= 0x61 && c <= 0x7a) {
				w.ansi = false
			}
		} else if inGroup(w.Newline, c) {
			w.flushDeferred()
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
			// Space = natural break point.
			// If deferred: decide where to break.
			if w.hasDeferred() {
				// The deferred chars + current line fit? Break before deferred.
				if w.lineLen+w.deferredWidth <= w.Limit {
					// Deferred chars start a new line
					w.breakDeferred()
				} else {
					// Absorb deferred into current line
					w.flushDeferred()
				}
			}
			w.addWord()
			_, _ = w.space.WriteRune(c)
			prevCJK = false
		} else if inGroup(w.Breakpoints, c) {
			if w.hasDeferred() {
				w.breakDeferred()
			}
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
				prohibitBreak := lineEndProhibited[c]

				if w.hasDeferred() {
					// Already in kinsoku lookahead.
					// Add this char to deferred buffer.
					w.deferred.WriteRune(c)
					rw := 2 // CJK width
					w.deferredWidth += rw

					if lineStartProhibited[c] {
						// Can't break before this char — keep deferring
					} else if w.lineLen+w.deferredWidth <= w.Limit {
						// Deferred content fits on current line — absorb it
						w.flushDeferred()
					} else if w.lineLen+w.deferredWidth <= w.Limit+20 {
						// Deferred content slightly exceeds — keep deferring for now
						// (look for a better break point)
					} else {
						// Deferred content way exceeds — break before deferred
						w.breakDeferred()
					}
				} else if !prohibitBreak && w.currentLineLen() > w.Limit &&
					w.word.PrintableRuneWidth() < w.Limit {
					// Soft limit exceeded — start kinsoku deferral.
					// Move the current char from word to deferred buffer.
					w.deferred.WriteRune(c)
					w.deferredWidth += 2
					w.word.Reset() // remove char from word
				} else {
					w.addWord()
				}
			} else if w.currentLineLen() > w.Limit &&
				w.word.PrintableRuneWidth() < w.Limit {
				if w.hasDeferred() {
					w.breakDeferred()
				}
				w.addNewLine()
			}
		}
	}

	return len(b), nil
}

// Close finishes the word-wrap operation.
func (w *WordWrap) Close() error {
	if w.hasDeferred() {
		// Decide: absorb or break
		if w.lineLen+w.deferredWidth <= w.Limit+20 {
			w.flushDeferred()
		} else {
			w.breakDeferred()
		}
	}
	w.addWord()
	return nil
}

func (w *WordWrap) Bytes() []byte {
	return w.buf.Bytes()
}

func (w *WordWrap) String() string {
	return w.buf.String()
}
