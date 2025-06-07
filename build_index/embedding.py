from FlagEmbedding import BGEM3FlagModel
import numpy as np

model = BGEM3FlagModel(
    'BAAI/bge-m3',
    cache_dir='./',
    use_fp16=True
)

def get_embedding(text: str, dim: int = 384) -> np.ndarray:
    return model.encode([text], batch_size=1, max_length=512,)['dense_vecs'][0]

if __name__ == "__main__":
    sentences_1 = ["What is BGE M3?", "Defination of BM25"]
    sentences_2 = ["BGE M3 is an embedding model supporting dense retrieval, lexical matching and multi-vector interaction.", 
                "BM25 is a bag-of-words retrieval function that ranks a set of documents based on the query terms appearing in each document"]

    embeddings_1 = model.encode(sentences_1, 
                                batch_size=12, 
                                max_length=512,
                                )['dense_vecs']
    embeddings_2 = model.encode(sentences_2)['dense_vecs']
    similarity = embeddings_1 @ embeddings_2.T
    print(similarity)
