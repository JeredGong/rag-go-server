// go-server/internal/model/model.go
package model

// RagRequest 是 /rag 接口的请求体
type RagRequest struct {
	UserQuestion string `json:"userQuestion"`
	Catagory     int    `json:"catagory"`
}

// RagResponse 是统一的响应结构
type RagResponse struct {
	Status string                 `json:"status"`
	Data   map[string]interface{} `json:"data"`
}

// CourseRecommendation 是 LLM 解析后的课程推荐结果
type CourseRecommendation struct {
	Course string `json:"course"`
	Reason string `json:"reason"`
}

// SepToken 用于从 LLM 输出中截断 JSON 结果
const SepToken = "<|Result|>"

// SystemPrompt 是 DeepSeek 的 system 提示词
const SystemPrompt = `你是一个课程选择助手。
在用户的输入部分，你会得到一个json格式的字符串，叫做课程列表，以及一段查询。
json格式的字符串是一个列表，列表中的每个元素是一个字符串，描述了一个课程。
查询就是一个朴素的字符串，是用户的选课需求。
你的任务是：从课程列表中选择一到三门课程作为用户的选课推荐，要求尽可能满足用户的选课需求。
请注意，课程列表可能混入一些随机数据，也可能是一个空列表，没有可用或满足要求课程的情况下你可以不推荐任何课程。
你可以输出任何思考过程，但是最终需要形式化的给出结果。具体地说，你可以先输出任何东西，比如解析用户的需求，
分析提供的课程列表等。然后你需要输出一个特别标志 <|Result|>，在该标志后面是一个json格式的列表。列表中的
每个元素是一个字典，包含"course"和"reason"两个项目。理由应该尽可能简要。
具体地说，你的输出应该保持如下格式：

这里是你的分析过程。<|Result|>
[{"course": "你推荐课程的名称1", "reason": "你推荐课程的理由1"},
 {"course": "你推荐课程的名称2", "reason": "你推荐课程的理由2"},
 {"course": "你推荐课程的名称3", "reason": "你推荐课程的理由3"}]

以下是用户的输入`
