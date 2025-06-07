import argparse
from tqdm import tqdm
import pandas as pd
import json
import numpy as np
import faiss
import os

from embedding import get_embedding

QUERY_FMT = '''
课程名称：{}。
授课教师：{}。
课程内容与评价：{}。
考勤与平时作业：{}。
期末考核方式：{}。
评价填写人成绩：{}。
'''.strip()

STR_TO_CLASS = {
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


def parseDict(row: dict) -> tuple[str, str]:
    return QUERY_FMT.format(
        row["课程名称"], 
        row["授课老师"], 
        row["课程内容与评价"], 
        row["考勤与平时作业"], 
        row["期末考核方式"],
        row["课程成绩"],
    ), STR_TO_CLASS.get(row["课程属性"], 0)

# 主处理函数
def build_rag_database(csv_path: str, db_path: str, embedding_dim: int = 1024):
    df = pd.read_csv(csv_path, encoding="gbk")
    embeddings = []
    metadata = []

    for _, row in tqdm(df.iterrows()):
        row_dict = row.to_dict()
        content, catagory = parseDict(row_dict)
        vec = get_embedding(content, embedding_dim)
        embeddings.append(vec)
        metadata.append({"text": content, "catagory": catagory})

    embeddings_np = np.vstack(embeddings)
    index = faiss.IndexFlatL2(embedding_dim)
    index.add(embeddings_np)

    os.makedirs(db_path, exist_ok=True)
    faiss.write_index(index, os.path.join(db_path, "faiss.index"))
    with open(os.path.join(db_path, "metadata.json"), "w", encoding="utf-8") as f:
        json.dump(metadata, f, ensure_ascii=False, indent=2)

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Build FAISS RAG database from CSV")
    parser.add_argument("--csv", type=str, required=True, help="Path to input CSV file")
    parser.add_argument("--db", type=str, default="./db", help="Output directory for FAISS index and metadata")
    args = parser.parse_args()

    build_rag_database(args.csv, args.db)