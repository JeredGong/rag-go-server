# API 接口文档

本文档详细描述了 RAG Go Server 提供的 HTTP API 接口。

## 目录

- [基本信息](#基本信息)
- [认证与限流](#认证与限流)
- [接口列表](#接口列表)
  - [POST /rag](#post-rag)
- [错误处理](#错误处理)
- [使用示例](#使用示例)

---

## 基本信息

### 服务地址

- **开发环境**: `http://127.0.0.1:8091`
- **生产环境**: 根据实际部署配置

### 数据格式

- **请求格式**: `application/json`
- **响应格式**: `application/json`
- **字符编码**: `UTF-8`

### API 版本

- **当前版本**: v1.0
- **兼容性**: 向后兼容

---

## 认证与限流

### 设备指纹认证

所有 API 请求都需要在 HTTP 请求头中携带设备指纹：

```http
X-Device-Fingerprint: your-unique-device-id
```

**设备指纹生成建议**：
- 前端可使用 [FingerprintJS](https://github.com/fingerprintjs/fingerprintjs) 等库生成
- 小程序可使用 `wx.getSystemInfo()` 获取设备信息后计算哈希
- 后端可使用 User-Agent + IP 地址组合
- 建议使用 UUID v4 格式以确保唯一性

### 访问频率限制

- **限流策略**: 基于设备指纹的固定窗口计数
- **配额上限**: 每个设备每周 10 次请求（可配置）
- **重置周期**: 每周四凌晨 00:00 自动重置
- **超限响应**: HTTP 429 Too Many Requests

**配额查询**：
- 当前 API 不支持查询剩余配额
- 建议前端自行记录已使用次数

---

## 接口列表

### POST /rag

根据用户的自然语言问题，检索并推荐相关课程。

#### 请求参数

**HTTP Method**: `POST`

**Content-Type**: `application/json`

**Headers**:

| 参数名                  | 类型   | 必填 | 说明                               |
| ----------------------- | ------ | ---- | ---------------------------------- |
| X-Device-Fingerprint    | string | 是   | 设备指纹，用于限流和身份识别       |
| Content-Type            | string | 是   | 必须为 `application/json`          |

**Request Body**:

| 参数名       | 类型   | 必填 | 说明                                       |
| ------------ | ------ | ---- | ------------------------------------------ |
| userQuestion | string | 是   | 用户的自然语言问题，建议 10-200 字符      |
| catagory     | int    | 是   | 课程分类筛选条件，0 表示不限制             |

**catagory 枚举值**：

| 值  | 说明                       |
| --- | -------------------------- |
| 0   | 不指定分类（返回所有类型） |
| 1   | 体育课                     |
| 2   | 通识选修课（公选课）       |
| 3   | 公共必修课                 |
| 4   | 专业课程                   |
| 5   | 通识必修课（导引课）       |
| 6   | 英语课                     |

#### 请求示例

```bash
curl -X POST http://127.0.0.1:8091/rag \
  -H "X-Device-Fingerprint: 123e4567-e89b-12d3-a456-426614174000" \
  -H "Content-Type: application/json" \
  -d '{
    "userQuestion": "我想选一些没有期末考试的课程",
    "catagory": 0
  }'
```

#### 响应参数

**成功响应** (HTTP 200):

```json
{
  "status": "success",
  "data": {
    "recommendations": [
      {
        "course": "公共艺术赏析",
        "reason": "课程内容轻松，采用论文考核，无期末考试"
      },
      {
        "course": "摄影基础",
        "reason": "实践类课程，以作品集代替考试"
      }
    ]
  }
}
```

**响应字段说明**：

| 字段                      | 类型   | 说明                               |
| ------------------------- | ------ | ---------------------------------- |
| status                    | string | 请求状态，`success` 或 `error`     |
| data                      | object | 数据载荷                           |
| data.recommendations      | array  | 推荐课程列表                       |
| data.recommendations[].course | string | 课程名称                           |
| data.recommendations[].reason | string | 推荐理由                           |

**注意事项**：
- `recommendations` 数组长度为 0-3
- 如果没有合适的课程，返回空数组 `[]`
- 推荐理由会基于检索到的课程信息生成

---

## 错误处理

### 错误响应格式

所有错误都遵循统一的响应格式：

```json
{
  "status": "error",
  "data": {
    "message": "错误描述信息"
  }
}
```

### 常见错误码

#### 400 Bad Request

**场景**：请求参数错误

```json
{
  "status": "error",
  "data": {
    "message": "缺少设备指纹"
  }
}
```

**常见原因**：
- 缺少 `X-Device-Fingerprint` 请求头
- 请求体 JSON 格式错误
- 必填字段缺失

**解决方法**：
- 检查请求头是否包含设备指纹
- 验证 JSON 格式是否正确
- 确保 `userQuestion` 和 `catagory` 都已提供

#### 429 Too Many Requests

**场景**：超出访问配额

```json
{
  "status": "error",
  "data": {
    "message": "访问次数已用完，请稍后再试"
  }
}
```

**常见原因**：
- 当前设备本周已使用完 10 次配额
- 配额将在下周四 00:00 重置

**解决方法**：
- 等待配额重置（每周四凌晨）
- 联系管理员申请额外配额
- 使用不同的设备指纹（不推荐）

#### 500 Internal Server Error

**场景**：服务器内部错误

```json
{
  "status": "error",
  "data": {
    "message": "LLM 调用失败: connection timeout"
  }
}
```

**常见原因**：
- DeepSeek API 调用失败
- Qdrant 向量检索超时
- Redis 连接中断
- 向量嵌入服务不可用

**解决方法**：
- 重试请求
- 检查服务端日志
- 验证外部服务（Qdrant、Redis、DeepSeek）状态

---

## 使用示例

### JavaScript (Fetch API)

```javascript
async function getCourseRecommendations(question, category = 0) {
  const deviceId = getDeviceFingerprint(); // 获取设备指纹

  const response = await fetch('http://127.0.0.1:8091/rag', {
    method: 'POST',
    headers: {
      'X-Device-Fingerprint': deviceId,
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      userQuestion: question,
      catagory: category
    })
  });

  const data = await response.json();

  if (data.status === 'success') {
    return data.data.recommendations;
  } else {
    throw new Error(data.data.message);
  }
}

// 使用示例
getCourseRecommendations("我想选轻松一点的公选课")
  .then(courses => {
    courses.forEach(course => {
      console.log(`${course.course}: ${course.reason}`);
    });
  })
  .catch(error => {
    console.error('推荐失败:', error.message);
  });
```

### Python (requests)

```python
import requests
import uuid

def get_course_recommendations(question, category=0):
    """获取课程推荐"""
    
    # 生成设备指纹（实际应用中应持久化）
    device_id = str(uuid.uuid4())
    
    url = 'http://127.0.0.1:8091/rag'
    headers = {
        'X-Device-Fingerprint': device_id,
        'Content-Type': 'application/json'
    }
    payload = {
        'userQuestion': question,
        'catagory': category
    }
    
    response = requests.post(url, json=payload, headers=headers)
    data = response.json()
    
    if response.status_code == 200 and data['status'] == 'success':
        return data['data']['recommendations']
    else:
        raise Exception(f"请求失败: {data['data']['message']}")

# 使用示例
try:
    courses = get_course_recommendations("推荐一些作业少的课")
    for course in courses:
        print(f"{course['course']}: {course['reason']}")
except Exception as e:
    print(f"错误: {e}")
```

### 微信小程序

```javascript
// pages/recommend/recommend.js
Page({
  data: {
    question: '',
    recommendations: []
  },

  // 获取推荐
  getRecommendations() {
    const deviceId = wx.getStorageSync('deviceId') || this.generateDeviceId();
    
    wx.request({
      url: 'http://your-server.com/rag',
      method: 'POST',
      header: {
        'X-Device-Fingerprint': deviceId,
        'Content-Type': 'application/json'
      },
      data: {
        userQuestion: this.data.question,
        catagory: 0
      },
      success: (res) => {
        if (res.data.status === 'success') {
          this.setData({
            recommendations: res.data.data.recommendations
          });
        } else {
          wx.showToast({
            title: res.data.data.message,
            icon: 'none'
          });
        }
      },
      fail: (error) => {
        wx.showToast({
          title: '网络请求失败',
          icon: 'none'
        });
      }
    });
  },

  // 生成设备指纹
  generateDeviceId() {
    const systemInfo = wx.getSystemInfoSync();
    const deviceId = `${systemInfo.model}_${systemInfo.system}_${Date.now()}`;
    wx.setStorageSync('deviceId', deviceId);
    return deviceId;
  }
});
```

---

## 最佳实践

### 1. 设备指纹管理

```javascript
// 推荐：持久化存储设备指纹
function getOrCreateDeviceFingerprint() {
  let fingerprint = localStorage.getItem('device_fingerprint');
  
  if (!fingerprint) {
    // 首次访问，生成新指纹
    fingerprint = generateUUID();
    localStorage.setItem('device_fingerprint', fingerprint);
  }
  
  return fingerprint;
}

function generateUUID() {
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
    const r = Math.random() * 16 | 0;
    const v = c === 'x' ? r : (r & 0x3 | 0x8);
    return v.toString(16);
  });
}
```

### 2. 错误重试策略

```javascript
async function fetchWithRetry(url, options, maxRetries = 3) {
  for (let i = 0; i < maxRetries; i++) {
    try {
      const response = await fetch(url, options);
      
      if (response.ok) {
        return await response.json();
      }
      
      // 429 错误不重试
      if (response.status === 429) {
        throw new Error('访问配额已用尽');
      }
      
    } catch (error) {
      if (i === maxRetries - 1) throw error;
      
      // 指数退避
      await new Promise(resolve => setTimeout(resolve, Math.pow(2, i) * 1000));
    }
  }
}
```

### 3. 用户输入验证

```javascript
function validateUserInput(question) {
  // 长度检查
  if (!question || question.trim().length === 0) {
    throw new Error('问题不能为空');
  }
  
  if (question.length > 200) {
    throw new Error('问题过长，请控制在 200 字以内');
  }
  
  // 敏感词过滤（根据实际需求）
  const sensitiveWords = ['测试', '垃圾'];
  const hasSensitive = sensitiveWords.some(word => question.includes(word));
  if (hasSensitive) {
    throw new Error('问题包含敏感词');
  }
  
  return question.trim();
}
```

---

## 变更日志

### v1.0 (2024-11-17)

- 初始版本发布
- 支持基于自然语言的课程推荐
- 实现设备指纹限流机制
- 支持课程分类筛选

---

## 技术支持

如有问题或建议，请通过以下方式联系：

- 📧 Email: support@example.com
- 💬 Issue: [GitHub Issues](https://github.com/your-repo/issues)
- 📖 文档: [项目主页](https://github.com/your-repo)

