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

QUERY_FMT = (
    "课程名称：{}。\n"
    "授课教师：{}。\n"
    "课程内容与评价：{}。\n"
    "考勤与平时作业：{}。\n"
    "期末考核方式：{}。\n"
    "评价填写人成绩：{}。"
)

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
# RAG 构造辅助函数
# =========================

def parseDict(row: dict) -> tuple[str, int]:
    """
    将 CSV 行转换为 RAG 文本格式以及课程类别编号。
    返回：
        formatted_text: str
        class_id: int
    """
    formatted_text = QUERY_FMT.format(
        row.get("课程名称", ""),
        row.get("授课老师", ""),
        row.get("课程内容与评价", ""),
        row.get("考勤与平时作业", ""),
        row.get("期末考核方式", ""),
        row.get("课程成绩", "")
    )

    course_type = row.get("课程属性", "不指定")
    class_id = COURSE_TYPE_TO_CLASS.get(course_type, 0)

    return formatted_text, class_id


# =========================
# 主处理函数
# =========================

def build_rag_database(csv_path: str, db_path: str, embedding_dim: int = 1024):
    """
    从 CSV 构建 RAG 数据库，包括：
        - 生成嵌入 embedding
        - 构建 FAISS 索引
        - 保存元数据到 JSON

    参数：
        csv_path: 输入 CSV 文件路径
        db_path: 输出数据库目录
        embedding_dim: 向量维度
    """

    if not os.path.exists(csv_path):
        raise FileNotFoundError(f"CSV 文件不存在：{csv_path}")

    # 优先尝试 GBK，失败时转 UTF-8
    try:
        df = pd.read_csv(csv_path, encoding="gbk")
    except UnicodeDecodeError:
        df = pd.read_csv(csv_path, encoding="utf-8")

    embeddings = []
    metadata = []

    print("正在生成 embeddings 并构建元数据……")

    for _, row in tqdm(df.iterrows(), total=len(df)):
        row_dict = row.to_dict()
        text, category = parseDict(row_dict)

        vec = get_embedding(text, embedding_dim)
        embeddings.append(vec)

        metadata.append({
            "text": text,
            "category": category,
        })

    # 构建 FAISS 索引
    embeddings_np = np.vstack(embeddings).astype("float32")
    index = faiss.IndexFlatL2(embedding_dim)
    index.add(embeddings_np)

    # 输出目录
    os.makedirs(db_path, exist_ok=True)

    faiss.write_index(index, os.path.join(db_path, "faiss.index"))

    with open(os.path.join(db_path, "metadata.json"), "w", encoding="utf-8") as f:
        json.dump(metadata, f, ensure_ascii=False, indent=2)

    print(f"RAG 数据库构建完成！已保存到：{db_path}")


# =========================
# CLI 部分
# =========================

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Build FAISS RAG database from CSV")
    parser.add_argument("--csv", type=str, required=True, help="Path to input CSV file")
    parser.add_argument("--db", type=str, default="./db", help="Output directory for FAISS index and metadata")
    args = parser.parse_args()

    build_rag_database(args.csv, args.db)
