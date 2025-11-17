package rag

import "errors"

// ErrRateLimitExceeded 表示设备配额已耗尽
var ErrRateLimitExceeded = errors.New("访问次数已用完，请稍后再试")
