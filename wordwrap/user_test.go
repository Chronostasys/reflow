package wordwrap

import (
	"strings"
	"testing"
	"unicode"
)

func TestIsCJK_RealCJK(t *testing.T) {
	cjkChars := "我做更严格的验证这样既有速度又有权威性你好世界"
	for _, r := range cjkChars {
		if !isCJK(r) {
			t.Errorf("expected %c (U+%04X) to be CJK", r, r)
		}
	}
}

func TestIsCJK_NoFalsePositives(t *testing.T) {
	nonCJK := "abcABC123-+/helloworld"
	for _, r := range nonCJK {
		if isCJK(r) {
			t.Errorf("expected %c (U+%04X) to NOT be CJK", r, r)
		}
	}
}

func TestCJK_NoProhibitedAtStart(t *testing.T) {
	texts := []struct {
		text  string
		limit int
	}{
		// Realistic limits where line breaking actually happens
		{"测试。新句子开始", 6},
		{"这是一个测试，后面还有更多内容", 10},
		{"验证。这样既有速度又有权威性", 10},
	}
	for _, tt := range texts {
		out := String(tt.text, tt.limit)
		for i, line := range strings.Split(out, "\n") {
			runes := []rune(strings.TrimRight(line, " "))
			if len(runes) == 0 { continue }
			first := runes[0]
			if lineStartProhibited[first] {
				t.Errorf("text=%q limit=%d: line %d starts with prohibited %c: %q",
					tt.text, tt.limit, i, first, line)
			}
		}
	}
}

func TestCJK_NoProhibitedAtEnd(t *testing.T) {
	texts := []struct {
		text  string
		limit int
	}{
		{"这是一个（测试数据", 6},
		{"验证（速度和权威性", 10},
	}
	for _, tt := range texts {
		out := String(tt.text, tt.limit)
		for i, line := range strings.Split(out, "\n") {
			runes := []rune(strings.TrimRight(line, " "))
			if len(runes) == 0 { continue }
			last := runes[len(runes)-1]
			if lineEndProhibited[last] {
				t.Errorf("text=%q limit=%d: line %d ends with prohibited %c: %q",
					tt.text, tt.limit, i, last, line)
			}
		}
	}
}

func TestCJK_RealGlamourText(t *testing.T) {
	text := "我的建议：先用 Aider Benchmark 做日常快速迭代（几分钟出结果），定期用 SWE-bench 小子集 做更严格的验证。这样既有速度又有权威性。"

	for _, limit := range []int{45, 50, 55, 60, 70, 80, 86} {
		out := String(text, limit)
		for i, line := range strings.Split(out, "\n") {
			runes := []rune(strings.TrimRight(line, " "))
			if len(runes) == 0 { continue }
			first := runes[0]
			if lineStartProhibited[first] {
				t.Errorf("limit %d: line %d starts with prohibited %c: %q", limit, i, first, line)
			}
			last := runes[len(runes)-1]
			if lineEndProhibited[last] {
				t.Errorf("limit %d: line %d ends with prohibited %c: %q", limit, i, last, line)
			}
			_ = unicode.Is(unicode.Han, first)
		}
	}
}
