package rag

import (
	"reflect"
	"testing"

	"rag-go-server/internal/model"
)

func TestParseLLMOutput_WithCodeFence(t *testing.T) {
	raw := `模型推理过程<|Result|>
```json
[
  {"course": "示例课程", "reason": "理由"}
]
```
`
	got, err := ParseLLMOutput(raw)
	if err != nil {
		t.Fatalf("ParseLLMOutput returned error: %v", err)
	}
	want := []model.CourseRecommendation{{Course: "示例课程", Reason: "理由"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestParseLLMOutput_SingleObject(t *testing.T) {
	raw := `analysis<|Result|>{"course": "A", "reason": "B"}`
	got, err := ParseLLMOutput(raw)
	if err != nil {
		t.Fatalf("ParseLLMOutput returned error: %v", err)
	}
	if len(got) != 1 || got[0].Course != "A" {
		t.Fatalf("expected single recommendation, got %+v", got)
	}
}
