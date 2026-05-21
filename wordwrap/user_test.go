package wordwrap

import (
	"fmt"
	"testing"
)

func TestCJK_RealGlamourText(t *testing.T) {
	text := "我的建议：先用 Aider Benchmark 做日常快速迭代（几分钟出结果），定期用 SWE-bench 小子集 做更严格的验证。这样既有速度又有权威性。"
	for _, limit := range []int{45, 50, 55} {
		got := String(text, limit)
		fmt.Printf("\n=== Limit %d ===\n%s\n", limit, got)
	}
	// Key assertion: "这样既有速度又有权威性" should NOT be split into
	// "这样既有" + "速度又有权威性" at these widths
	for _, limit := range []int{45, 50, 55} {
		got := String(text, limit)
		if got == "" {
			t.Errorf("Limit %d: empty output", limit)
		}
		// Verify no ugly mid-sentence break in the last clause
		bad := "这样既有\n速度又有权威性"
		if len(got) >= len(bad) {
			for i := 0; i <= len(got)-len(bad); i++ {
				if got[i:i+len(bad)] == bad {
					t.Errorf("Limit %d: ugly break '这样既有\\n速度又有权威性'", limit)
				}
			}
		}
	}
}

func TestIsCJK_NoFalsePositives(t *testing.T) {
	notCJK := []rune{'a', 'Z', '0', '-', 'ⁿ', 'Ⅰ', 'ʰ'}
	for _, r := range notCJK {
		if isCJK(r) {
			t.Errorf("isCJK(%q U+%04X) = true, want false", string(r), r)
		}
	}
}

func TestIsCJK_RealCJK(t *testing.T) {
	cjk := []rune{'中', '国', '日', '語', '한', '、', '。', '，'}
	for _, r := range cjk {
		if !isCJK(r) {
			t.Errorf("isCJK(%q) = false, want true", string(r))
		}
	}
}
