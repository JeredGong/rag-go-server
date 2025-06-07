import faiss
import json
from qdrant_client import QdrantClient

# ======= 配置参数 =======
QDRANT_URL = "https://a7dcca84-9674-46dd-b955-2d599dac27e9.us-west-1-0.aws.cloud.qdrant.io:6333"
QDRANT_API_KEY = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJhY2Nlc3MiOiJtIn0.ph3P7vJNPkg9I2IEI2QcfT0czW9nuX_3d3a2q0I77rY"
COLLECTION_NAME = "WHUCoursesDB"

# ======= 读取本地向量做查询用 =======
index = faiss.read_index("faiss.index")
vectors = index.reconstruct_n(0, 1)  # 读取第一个向量
query_vector = vectors[0].tolist()

# ======= 初始化 Qdrant 客户端 =======
client = QdrantClient(
    url=QDRANT_URL,
    api_key=QDRANT_API_KEY
)

# ======= 执行相似搜索 =======
results = client.search(
    collection_name=COLLECTION_NAME,
    query_vector=query_vector,
    limit=3  # 返回最相似的前3个
)

# ======= 输出结果 =======
print("✅ Qdrant 查询成功，Top-3 相似结果：\n")
for i, result in enumerate(results):
    print(f"🔹 第{i+1}个结果（Score: {result.score:.4f}）")
    print(f"分类：{result.payload.get('catagory')}")
    print(f"内容片段：{result.payload.get('text')[:100]}...\n")
