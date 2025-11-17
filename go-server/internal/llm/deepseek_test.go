package llm

import (
	"strings"
	"testing"
)

func TestSanitizeCourseText(t *testing.T) {
	long := "课程" + strings.Repeat("测", maxCourseContextRunes+10)
	got := sanitizeCourseText(long)
	if len([]rune(got)) > maxCourseContextRunes+3 {
		t.Fatalf("text not truncated properly, length=%d", len([]rune(got)))
	}

	short := "  数据结构  "
	if sanitizeCourseText(short) != "数据结构" {
		t.Fatalf("expected trimmed text")
	}
}
