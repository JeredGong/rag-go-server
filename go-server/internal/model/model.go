// go-server/internal/model/model.go
//
// 数据模型定义模块
//
// 本模块定义了整个 RAG 服务中使用的核心数据结构，包括：
// 1. HTTP 请求和响应的结构体
// 2. 内部业务逻辑使用的数据模型
// 3. 与大语言模型交互相关的常量配置
//
// 将数据模型集中定义在一个包中，便于统一管理和维护。
package model

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// RagRequest 表示客户端发送的课程推荐请求
//
// 该结构体对应 POST /rag 接口的请求体，前端需按此格式发送 JSON 数据。
type RagRequest struct {
	// UserQuestion 用户的自然语言问题
	// 示例："我想选一些没有期末考试的课程"
	UserQuestion string `json:"userQuestion"`

	// Catagory 课程分类筛选条件
	// 0 表示不限制分类，其他数值对应具体的课程类型：
	//   1 - 体育课
	//   2 - 通识选修课（公选课）
	//   3 - 公共必修课（高数、线代、大物和思政课等）
	//   4 - 专业课程
	//   5 - 通识必修课（导引课）
	//   6 - 英语课
	Catagory int `json:"catagory"`
}

// Normalize 对字段进行基础清洗
func (r *RagRequest) Normalize() {
	r.UserQuestion = strings.TrimSpace(r.UserQuestion)
}

// Validate 校验字段合法性
func (r RagRequest) Validate() error {
	if r.UserQuestion == "" {
		return NewValidationError("userQuestion", "问题内容不能为空")
	}
	if utf8.RuneCountInString(r.UserQuestion) > MaxQuestionRunes {
		return NewValidationError("userQuestion", "问题内容过长")
	}
	if r.Catagory < 0 {
		return NewValidationError("catagory", "catagory 不能为负数")
	}
	return nil
}

// RagResponse 表示服务端返回的统一响应格式
//
// 所有 API 响应都遵循这个结构，便于前端统一处理。
type RagResponse struct {
	// Status 请求状态标识
	// 取值：
	//   - "success": 请求成功
	//   - "error": 请求失败
	Status string `json:"status"`
	
	// Data 响应数据的载荷
	// 成功时包含业务数据（如推荐结果列表）
	// 失败时包含错误信息（如 {"message": "错误描述"}）
	// 使用 map 提供灵活性，支持不同的响应结构
	Data map[string]interface{} `json:"data"`
}

// CourseRecommendation 表示一条课程推荐记录
//
// 该结构体由 LLM 的 JSON 输出解析而来，包含课程名称和推荐理由。
type CourseRecommendation struct {
	// Course 推荐的课程名称
	// 示例："公共艺术赏析"
	Course string `json:"course"`
	
	// Reason 推荐该课程的理由
	// 应简明扼要，例如："课程内容轻松，无期末考试"
	Reason string `json:"reason"`
}

// SepToken 是 LLM 输出中的分隔符标记
//
// LLM 会在自由文本解释之后输出这个特殊标记，紧跟着是结构化的 JSON 结果。
// 通过这个标记，我们可以将 LLM 的"思考过程"和"最终结果"分离开来。
//
// 示例输出格式：
//   用户想要无期末考试的课程。根据课程列表，我推荐以下三门：<|Result|>
//   [{"course": "课程A", "reason": "无考试"}, ...]
const SepToken = "<|Result|>"

// MaxQuestionRunes 限制单次问题长度
const MaxQuestionRunes = 1024

// SystemPrompt 是发送给 DeepSeek 大模型的系统提示词
//
// 该提示词定义了 AI 的角色、任务要求和输出格式规范。
// 核心要点：
// 1. 角色定位：课程选择助手
// 2. 输入格式：课程列表（JSON）+ 用户查询（自然语言）
// 3. 输出要求：先自由分析，再输出 <|Result|> 标记，最后是 JSON 数组
// 4. 输出结构：每个推荐包含 "course" 和 "reason" 两个字段
// 5. 推荐数量：1-3 门课程，允许不推荐
//
// 通过清晰的提示词，确保 LLM 输出的一致性和可解析性。
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

// ValidationError 表示请求字段校验失败
type ValidationError struct {
	Field   string
	Message string
}

func (v ValidationError) Error() string {
	return v.Field + ": " + v.Message
}

// NewValidationError 创建一个 ValidationError
func NewValidationError(field, msg string) error {
	return ValidationError{Field: field, Message: msg}
}

// IsValidationError 判断错误是否为 ValidationError
func IsValidationError(err error) bool {
	var target ValidationError
	return errors.As(err, &target)
}
