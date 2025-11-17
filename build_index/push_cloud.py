import faiss
import json
from qdrant_client import QdrantClient

# =========================
# 配置参数
# =========================

QDRANT_URL = "https://a7dcca84-9674-46dd-b955-2d599dac27e9.us-west-1-0.aws.cloud.qdrant.io:6333"
QDRANT_API_KEY = (
    "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJhY2Nlc3MiOiJtIn0.ph3P7vJNPkg9I2IEI2QcfT0czW9nuX_3d3a2q0I77rY"
)
COLLECTION_NAME = "WHUCoursesDB"

# =========================
# 从本地 FAISS 读取向量作为查询
# =========================

try:
    index = faiss.read_index("faiss.index")
except Exception as e:
    raise RuntimeError(f"无法读取 faiss.index：{e}")

# 读取第一个向量作为查询向量
try:
    reconstructed = index.reconstruct_n(0, 1)
    query_vector = reconstructed[0].tolist()
except Exception as e:
    raise RuntimeError(f"从 FAISS 重构向量失败：{e}")

# =========================
# 初始化 Qdrant 客户端
# =========================

client = QdrantClient(
    url=QDRANT_URL,
    api_key=QDRANT_API_KEY
)

# =========================
# 执行相似搜索
# =========================

try:
    results = client.search(
        collection_name=COLLECTION_NAME,
        query_vector=query_vector,
        limit=3
    )
except Exception as e:
    raise RuntimeError(f"Qdrant 查询失败：{e}")

# =========================
# 输出 Top-K 查询结果
# =========================

print("\n=== 🎯 Qdrant Top-3 最相似课程查询结果 ===\n")

for idx, result in enumerate(results, start=1):
    payload = result.payload or {}
    text_snippet = payload.get("text", "")
    category = payload.get("catagory")

    # 控制摘要长度
    snippet = text_snippet[:120].replace("\n", " ") + ("..." if len(text_snippet) > 120 else "")

    print(f"🔹 结果 {idx}")
    print(f"   ▸ 相似度分数：{result.score:.4f}")
    print(f"   ▸ 所属类别：{category}")
    print(f"   ▸ 内容摘要：{snippet}\n")

print("=== ✅ 查询结束 ===\n")
