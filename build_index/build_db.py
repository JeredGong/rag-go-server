"""
build_db.py - 课程向量数据库构建脚本

本脚本负责从课程评价 CSV 文件构建向量数据库，主要步骤包括：
1. 读取 CSV 格式的课程数据
2. 将每门课程的信息格式化为结构化文本
3. 调用 BGE-M3 模型生成向量嵌入
4. 使用 FAISS 构建向量索引
5. 保存索引文件和元数据

生成的数据库供在线服务使用，支持基于语义相似度的课程检索。

使用方法：
    python build_db.py --csv CouresesData.csv --db ./db

依赖库：
    - faiss-cpu/faiss-gpu: 向量索引和检索
    - pandas: CSV 数据处理
    - numpy: 数值计算
    - FlagEmbedding: BGE-M3 向量化模型
"""

import argparse
import json
import os

import faiss
import numpy as np
import pandas as pd
from tqdm import tqdm

from embedding import get_embedding

# =========================
# 常量与格式模板
# =========================

# 课程文本格式化模板
# 将课程的多个字段组织成结构化的文本描述，便于向量化和检索
QUERY_FMT = (
    "课程名称：{}。\n"
    "授课教师：{}。\n"
    "课程内容与评价：{}。\n"
    "考勤与平时作业：{}。\n"
    "期末考核方式：{}。\n"
    "评价填写人成绩：{}。"
)

# 课程类型映射表
# 将课程属性的文本描述映射为数值编号，便于后续过滤和分类
# 0 表示不指定类型，1-6 表示具体课程类别
COURSE_TYPE_TO_CLASS = {
    "不指定": 0,
    "体育课": 1,
    "通识选修课（公选课）": 2,
    "公共课": 3,
    "公共课（高数、线代、大物和思政课等）": 3,
    "公共必修课（高数、线代、大物和思政课等）": 3,
    "专业课程": 4,
    "通识必修课（导引课）": 5,
    "通识必修课（导引）": 5,
    "导引课（自科人文中国精神）": 5,
    "英语课": 6,
}


# =========================
# RAG 数据预处理函数
# =========================

def parseDict(row: dict) -> tuple[str, int]:
    """
    将 CSV 行数据解析为 RAG 系统所需的格式
    
    该函数完成两项任务：
    1. 将课程的多个字段拼接成一段结构化文本，用于向量化
    2. 将课程类型映射为数值编号，用于分类过滤
    
    参数：
        row: 一行课程数据（字典格式），包含以下字段：
            - 课程名称: 课程的官方名称
            - 授课老师: 教师姓名
            - 课程内容与评价: 学生对课程内容的评价
            - 考勤与平时作业: 考勤要求和作业负担
            - 期末考核方式: 考试/论文/开卷等
            - 课程成绩: 评价人的成绩等级
            - 课程属性: 课程类型（如体育课、公选课等）
    
    返回：
        formatted_text: 格式化后的课程描述文本
        class_id: 课程类型编号（0-6）
    
    示例：
        row = {
            "课程名称": "计算机网络",
            "授课老师": "张三",
            "课程内容与评价": "内容丰富，讲解清晰",
            ...
        }
        text, cat = parseDict(row)
        # text: "课程名称：计算机网络。\n授课教师：张三。\n..."
        # cat: 4
    """
    # 按照模板拼接各字段，缺失字段使用空字符串
    formatted_text = QUERY_FMT.format(
        row.get("课程名称", ""),
        row.get("授课老师", ""),
        row.get("课程内容与评价", ""),
        row.get("考勤与平时作业", ""),
        row.get("期末考核方式", ""),
        row.get("课程成绩", "")
    )

    # 获取课程类型并映射为数值编号
    course_type = row.get("课程属性", "不指定")
    class_id = COURSE_TYPE_TO_CLASS.get(course_type, 0)

    return formatted_text, class_id


# =========================
# 数据库构建主函数
# =========================

def build_rag_database(csv_path: str, db_path: str, embedding_dim: int = 1024):
    """
    构建 RAG 向量数据库
    
    该函数是整个索引构建流程的入口，完成以下步骤：
    
    1. 数据加载：
       - 从 CSV 文件读取课程数据
       - 自动处理 GBK/UTF-8 编码问题
    
    2. 向量生成：
       - 遍历每门课程，生成格式化文本
       - 调用 BGE-M3 模型生成 1024 维向量
    
    3. 索引构建：
       - 使用 FAISS IndexFlatL2 构建 L2 距离索引
       - 支持快速的向量相似度检索
    
    4. 持久化存储：
       - 保存 FAISS 索引到 faiss.index 文件
       - 保存课程元数据到 metadata.json 文件
    
    参数：
        csv_path: 输入 CSV 文件的路径，必须包含课程评价数据
        db_path: 输出目录路径，用于保存索引和元数据
        embedding_dim: 向量维度，BGE-M3 默认为 1024
    
    抛出：
        FileNotFoundError: 如果 CSV 文件不存在
        UnicodeDecodeError: 如果编码格式不支持（已自动处理 GBK/UTF-8）
    
    输出文件：
        {db_path}/faiss.index: FAISS 向量索引文件
        {db_path}/metadata.json: 课程元数据（文本和类别）
    
    示例：
        build_rag_database("courses.csv", "./db", embedding_dim=1024)
    """
    
    # ========================================
    # 阶段1: 数据加载与验证
    # ========================================
    
    if not os.path.exists(csv_path):
        raise FileNotFoundError(f"CSV 文件不存在：{csv_path}")

    # 尝试以 GBK 编码读取（中文 Excel 导出的 CSV 通常使用 GBK）
    # 如果失败则回退到 UTF-8
    try:
        df = pd.read_csv(csv_path, encoding="gbk")
    except UnicodeDecodeError:
        df = pd.read_csv(csv_path, encoding="utf-8")

    # 初始化向量和元数据列表
    embeddings = []
    metadata = []

    print("正在生成 embeddings 并构建元数据……")

    # ========================================
    # 阶段2: 遍历数据，生成向量和元数据
    # ========================================
    
    # 使用 tqdm 显示进度条，提升用户体验
    for _, row in tqdm(df.iterrows(), total=len(df), desc="处理课程"):
        row_dict = row.to_dict()
        
        # 将课程数据格式化为文本和类别
        text, category = parseDict(row_dict)

        # 调用 BGE-M3 模型生成向量嵌入
        vec = get_embedding(text, embedding_dim)
        embeddings.append(vec)

        # 保存元数据，用于在检索结果中展示课程信息
        metadata.append({
            "text": text,          # 完整的课程描述文本
            "category": category,  # 课程类型编号
        })

    # ========================================
    # 阶段3: 构建 FAISS 索引
    # ========================================
    
    # 将向量列表转换为 NumPy 数组
    # FAISS 要求输入为二维 float32 数组，形状为 (n_samples, embedding_dim)
    embeddings_np = np.vstack(embeddings).astype("float32")
    
    # 创建 L2 距离索引（欧氏距离）
    # IndexFlatL2 是暴力搜索索引，适合数据量较小的场景（< 10万）
    # 如需更高性能，可使用 IVF、HNSW 等近似索引
    index = faiss.IndexFlatL2(embedding_dim)
    
    # 将所有向量添加到索引中
    index.add(embeddings_np)

    # ========================================
    # 阶段4: 持久化存储
    # ========================================
    
    # 创建输出目录（如果不存在）
    os.makedirs(db_path, exist_ok=True)

    # 保存 FAISS 索引到文件
    faiss.write_index(index, os.path.join(db_path, "faiss.index"))

    # 保存元数据到 JSON 文件
    # ensure_ascii=False 确保中文正常显示
    # indent=2 提高可读性
    with open(os.path.join(db_path, "metadata.json"), "w", encoding="utf-8") as f:
        json.dump(metadata, f, ensure_ascii=False, indent=2)

    print(f"✅ RAG 数据库构建完成！已保存到：{db_path}")
    print(f"   - 索引文件: {os.path.join(db_path, 'faiss.index')}")
    print(f"   - 元数据文件: {os.path.join(db_path, 'metadata.json')}")
    print(f"   - 总课程数: {len(embeddings)}")


# =========================
# 命令行接口
# =========================

if __name__ == "__main__":
    # 创建命令行参数解析器
    parser = argparse.ArgumentParser(
        description="从 CSV 文件构建 RAG 向量数据库",
        epilog="示例: python build_db.py --csv CouresesData.csv --db ./db"
    )
    
    # 定义命令行参数
    parser.add_argument(
        "--csv",
        type=str,
        required=True,
        help="输入 CSV 文件路径（必需），包含课程评价数据"
    )
    parser.add_argument(
        "--db",
        type=str,
        default="./db",
        help="输出数据库目录路径（可选，默认为 ./db）"
    )
    
    # 解析参数并执行构建
    args = parser.parse_args()
    build_rag_database(args.csv, args.db)
