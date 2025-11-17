"""
push_cloud.py - Qdrant 云端索引测试脚本

本脚本用于验证 Qdrant 云端向量数据库的连接和查询功能。

主要功能：
1. 从本地 FAISS 索引中读取一个测试向量
2. 连接到 Qdrant Cloud 实例
3. 执行向量相似度搜索
4. 展示 Top-3 最相似的课程结果

使用场景：
- 验证 Qdrant 数据库是否正确部署和索引
- 测试向量检索的性能和准确性
- 调试索引构建流程

注意事项：
⚠️ 本脚本包含 API 密钥，仅供测试使用，请勿提交到公开仓库
⚠️ 生产环境应从环境变量读取敏感配置

使用方法：
    python push_cloud.py

依赖库：
    - faiss-cpu/faiss-gpu: 读取本地索引文件
    - qdrant-client: Qdrant 官方 Python 客户端
"""

import faiss
import json
from qdrant_client import QdrantClient

# =========================
# 配置参数
# =========================

# Qdrant Cloud 实例的访问地址
# 格式：https://{cluster-id}.{region}.aws.cloud.qdrant.io:6333
QDRANT_URL = "https://a7dcca84-9674-46dd-b955-2d599dac27e9.us-west-1-0.aws.cloud.qdrant.io:6333"

# Qdrant API 密钥，用于身份认证
# ⚠️ 该密钥应从环境变量中读取，避免硬编码到代码中
QDRANT_API_KEY = (
    "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJhY2Nlc3MiOiJtIn0.ph3P7vJNPkg9I2IEI2QcfT0czW9nuX_3d3a2q0I77rY"
)

# Qdrant 集合名称，对应课程向量数据库
# 该名称需要与索引构建和在线服务中使用的名称保持一致
COLLECTION_NAME = "WHUCoursesDB"

# =========================
# 阶段1: 从本地 FAISS 索引读取测试向量
# =========================

print("📖 正在读取本地 FAISS 索引...")

try:
    # 加载 FAISS 索引文件
    # 该文件由 build_db.py 生成
    index = faiss.read_index("faiss.index")
    print(f"✅ 成功加载索引，共 {index.ntotal} 个向量")
except Exception as e:
    raise RuntimeError(f"❌ 无法读取 faiss.index：{e}")

# 从索引中重构第一个向量作为查询向量
# 注意：IndexFlatL2 支持 reconstruct 操作，某些压缩索引可能不支持
try:
    reconstructed = index.reconstruct_n(0, 1)  # 重构索引 0 到 1 的向量
    query_vector = reconstructed[0].tolist()    # 转换为 Python 列表
    print(f"✅ 提取测试向量，维度：{len(query_vector)}")
except Exception as e:
    raise RuntimeError(f"❌ 从 FAISS 重构向量失败：{e}")

# =========================
# 阶段2: 初始化 Qdrant 客户端
# =========================

print("\n🔗 正在连接 Qdrant Cloud...")

try:
    client = QdrantClient(
        url=QDRANT_URL,
        api_key=QDRANT_API_KEY
    )
    print("✅ 成功连接到 Qdrant Cloud")
except Exception as e:
    raise RuntimeError(f"❌ Qdrant 连接失败：{e}")

# =========================
# 阶段3: 执行向量相似度搜索
# =========================

print(f"\n🔍 正在查询集合 '{COLLECTION_NAME}' 中最相似的 3 门课程...")

try:
    # 调用 Qdrant 的 search 方法
    # 参数说明：
    #   - collection_name: 要搜索的集合名称
    #   - query_vector: 查询向量（1024 维浮点数列表）
    #   - limit: 返回的最相似结果数量
    results = client.search(
        collection_name=COLLECTION_NAME,
        query_vector=query_vector,
        limit=3
    )
    print(f"✅ 查询成功，返回 {len(results)} 个结果\n")
except Exception as e:
    raise RuntimeError(f"❌ Qdrant 查询失败：{e}")

# =========================
# 阶段4: 格式化并输出查询结果
# =========================

print("=" * 60)
print("🎯 Qdrant Top-3 最相似课程查询结果")
print("=" * 60)
print()

for idx, result in enumerate(results, start=1):
    # 提取 payload 中的元数据
    payload = result.payload or {}
    text_snippet = payload.get("text", "（无文本内容）")
    category = payload.get("catagory", "未分类")

    # 截取文本摘要，避免输出过长
    # 将换行符替换为空格，提升可读性
    snippet = text_snippet[:120].replace("\n", " ")
    if len(text_snippet) > 120:
        snippet += "..."

    # 输出格式化的结果
    print(f"🔹 结果 {idx}")
    print(f"   ▸ 相似度分数：{result.score:.4f}")
    print(f"      （分数越低表示越相似，L2 距离）")
    print(f"   ▸ 所属类别：{category}")
    print(f"   ▸ 内容摘要：{snippet}")
    print()

print("=" * 60)
print("✅ 查询结束")
print("=" * 60)
print()

# =========================
# 性能与准确性评估
# =========================

print("💡 提示：")
print("  - 如果相似度分数过大（> 100），可能需要检查向量归一化")
print("  - 如果返回结果不相关，可能需要调整检索策略或重新训练模型")
print("  - 可通过调整 limit 参数查看更多结果")
