"""
embedding.py - 文本向量化模块

本模块封装了 BGE-M3 模型的调用接口，提供文本到向量的转换功能。
BGE-M3 是由智源研究院开发的多语言稠密检索模型，具有以下特点：

模型特性：
1. 支持中英文等 100+ 种语言
2. 向量维度：1024（可调整输出维度）
3. 最大输入长度：8192 tokens（本项目限制为 512）
4. 支持稠密检索、词汇匹配和多向量交互

适用场景：
- 语义相似度搜索
- 文本分类和聚类
- 跨语言信息检索

模型来源：
- HuggingFace: BAAI/bge-m3
- 论文: BGE M3-Embedding: Multi-Lingual, Multi-Functionality, Multi-Granularity Text Embeddings
"""

from FlagEmbedding import BGEM3FlagModel
import numpy as np

# ========================================
# 模型初始化
# ========================================

# 加载 BGE-M3 预训练模型
# 参数说明：
#   - 'BAAI/bge-m3': HuggingFace 模型标识符，会自动下载模型文件
#   - cache_dir: 模型缓存目录，'./' 表示当前目录，避免重复下载
#   - use_fp16: 使用半精度浮点数（FP16）加速推理，节省显存
#               在 GPU 上会显著提升速度，CPU 上无效果
model = BGEM3FlagModel(
    'BAAI/bge-m3',
    cache_dir='./',
    use_fp16=True
)


# ========================================
# 向量化函数
# ========================================

def get_embedding(text: str, dim: int = 384) -> np.ndarray:
    """
    将文本转换为稠密向量表示
    
    该函数是整个向量化流程的核心接口，供索引构建和在线检索调用。
    
    工作原理：
    1. 将输入文本分词（tokenization）
    2. 通过 Transformer 编码器提取语义特征
    3. 使用池化层（pooling）生成固定维度的向量
    4. 返回归一化后的向量表示
    
    参数：
        text: 待向量化的文本，可以是中文或英文
              建议长度在 512 tokens 以内，超长文本会被截断
        dim: 向量输出维度，默认 384（实际上 BGE-M3 输出 1024 维，
             这个参数在当前实现中未使用，保留用于未来扩展）
    
    返回：
        np.ndarray: 文本的向量表示，形状为 (1024,)，dtype 为 float32
    
    示例：
        vec = get_embedding("人工智能是什么？")
        print(vec.shape)  # (1024,)
        print(vec.dtype)  # float32
    
    性能考虑：
        - 单次调用耗时约 50-200ms（取决于硬件）
        - 建议批量处理以提高吞吐量（见下方示例）
        - GPU 加速下性能提升明显
    """
    # 调用模型的 encode 方法生成向量
    # 参数说明：
    #   - [text]: 输入文本列表，即使单个文本也需要包装成列表
    #   - batch_size=1: 批处理大小，单个文本设为 1
    #   - max_length=512: 最大 token 长度，超出部分会被截断
    #                     512 是速度和质量的平衡点
    # 返回值：
    #   - 字典格式：{'dense_vecs': array([[...]]), ...}
    #   - dense_vecs: 稠密向量，形状为 (batch_size, 1024)
    #   - [0]: 提取第一个（也是唯一一个）样本的向量
    return model.encode([text], batch_size=1, max_length=512)['dense_vecs'][0]


# ========================================
# 测试和使用示例
# ========================================

if __name__ == "__main__":
    """
    模块测试代码
    
    该部分展示了如何使用 BGE-M3 进行文本相似度计算：
    1. 准备两组句子
    2. 分别生成向量
    3. 计算余弦相似度矩阵
    """
    
    # 第一组句子（查询）
    sentences_1 = [
        "What is BGE M3?",           # 关于 BGE M3 的问题
        "Defination of BM25"         # 关于 BM25 的问题
    ]
    
    # 第二组句子（文档）
    sentences_2 = [
        # BGE M3 的详细解释
        "BGE M3 is an embedding model supporting dense retrieval, lexical matching and multi-vector interaction.", 
        # BM25 的详细解释
        "BM25 is a bag-of-words retrieval function that ranks a set of documents based on the query terms appearing in each document"
    ]

    # 生成查询向量（2 个句子 → 2 × 1024 矩阵）
    embeddings_1 = model.encode(
        sentences_1, 
        batch_size=12,      # 批处理大小，可根据显存调整
        max_length=512      # 最大输入长度
    )['dense_vecs']
    
    # 生成文档向量（2 个句子 → 2 × 1024 矩阵）
    embeddings_2 = model.encode(sentences_2)['dense_vecs']
    
    # 计算相似度矩阵
    # 使用矩阵乘法计算余弦相似度（向量已归一化）
    # 结果形状：(2, 2)
    #   - [0, 0]: sentences_1[0] 与 sentences_2[0] 的相似度
    #   - [0, 1]: sentences_1[0] 与 sentences_2[1] 的相似度
    #   - [1, 0]: sentences_1[1] 与 sentences_2[0] 的相似度
    #   - [1, 1]: sentences_1[1] 与 sentences_2[1] 的相似度
    similarity = embeddings_1 @ embeddings_2.T
    
    print("相似度矩阵:")
    print(similarity)
    print("\n解读:")
    print(f"'What is BGE M3?' 与 BGE M3 定义的相似度: {similarity[0, 0]:.4f}")
    print(f"'What is BGE M3?' 与 BM25 定义的相似度: {similarity[0, 1]:.4f}")
    print(f"'Defination of BM25' 与 BGE M3 定义的相似度: {similarity[1, 0]:.4f}")
    print(f"'Defination of BM25' 与 BM25 定义的相似度: {similarity[1, 1]:.4f}")
