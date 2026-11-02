import os
import pandas as pd
import numpy as np
import faiss
from transformers import AutoTokenizer, AutoModel
import torch

class SkinDisease:
    def __init__(self, model_path, index_folder, data_file_path):
        self.model_path = model_path
        self.index_folder = index_folder
        self.index_save_path = os.path.join(index_folder, "des.index")
        self.data_file_path = data_file_path
        self.docs, self.all_descriptions = self.load_data(data_file_path)
        self.tokenizer = None
        self.model = None
        self.index = None

        # 初始化模型和索引
        self.initialize_model_and_index()

    def load_data(self, file_path):
        df = pd.read_excel(file_path)
        texts = []
        all_descriptions = []
        for _, row in df.iterrows():
            # 只对名称进行向量化
            text = f"名称：{row['概述']}"
            texts.append(text)
            # 保存全部描述信息
            all_description = f"名称：{row['名称']}，概述：{row['概述']}，症状：{row['症状']}，治疗方法：{row['治疗方法']}"
            all_descriptions.append(all_description)
        return texts, all_descriptions

    def encode_texts(self, texts, device="cpu"):
        embeddings = []
        for text in texts:
            inputs = self.tokenizer(text, return_tensors='pt', truncation=True, padding=True).to(device)
            with torch.no_grad():
                outputs = self.model(**inputs)
            # 取 [CLS] 标记的输出作为句子嵌入
            embedding = outputs.last_hidden_state[:, 0, :].cpu().numpy().flatten()
            embeddings.append(embedding)
        return np.array(embeddings, dtype=np.float32)

    def initialize_model_and_index(self):
        try:
            # 加载分词器和模型
            self.tokenizer = AutoTokenizer.from_pretrained(self.model_path)
            self.model = AutoModel.from_pretrained(self.model_path)
        except Exception as e:
            print(f"模型加载失败: {e}")
            return

        # 检查索引文件是否存在
        if not os.path.exists(self.index_save_path):
            self.build_faiss_index()
        else:
            self.load_faiss_index()

    def build_faiss_index(self):
        # 对文本进行编码
        embeddings = self.encode_texts(self.docs)

        # 获取向量维度
        dim = embeddings.shape[1]
        # 创建 FAISS 索引
        self.index = faiss.IndexFlatL2(dim)
        # 将编码后的向量添加到索引中
        self.index.add(embeddings)

        # 检查保存索引的目录是否存在，不存在则创建
        index_dir = os.path.dirname(self.index_save_path)
        if not os.path.exists(index_dir):
            try:
                os.makedirs(index_dir)
                print(f"成功创建目录: {index_dir}")
            except Exception as e:
                print(f"创建目录失败: {e}")
                return

        try:
            # 保存 FAISS 索引
            faiss.write_index(self.index, self.index_save_path)
            print(f"索引文件已保存到 {self.index_save_path}")
        except Exception as e:
            print(f"索引文件保存失败: {e}")

    def load_faiss_index(self):
        try:
            self.index = faiss.read_index(self.index_save_path)
            print("描述索引已加载")
        except Exception as e:
            print(f"索引加载失败: {e}")

    def search(self, query_text, k=2):
        if self.index is None:
            print("索引未初始化")
            return

        # 对查询文本进行编码
        query_embedding = self.encode_texts([query_text])

        # 搜索最相似的 k 个结果
        distances, indices = self.index.search(query_embedding, k)

        results = []
        for i in range(k):
            results.append({
                "rank": i + 1,
                "distance": distances[0][i],
                "description": self.all_descriptions[indices[0][i]]
            })

        return results


# 示例用法
if __name__ == "__main__":
    # 预训练模型路径
    model_path = r"D:/学习资料/实验室/皮肤病/skin_disease_2.0/model"
    # 索引保存文件夹
    index_folder = r"D:\faiss_index"
    # 数据文件路径
    data_file_path = r'D:\学习资料\实验室\皮肤病\skin_disease_2.0\皮肤病材料\皮肤病_丰富版_新增.xlsx'

    # 初始化搜索器
    search_engine = SkinDisease(model_path, index_folder, data_file_path)

    # 进行查询
    query_text = input("请输入疾病名称：")
    results = search_engine.search(query_text)

    # 输出结果
    print(f"查询文本: {query_text}")
    print("最相似的结果:")
    for result in results:
        print(f"排名 {result['rank']}: 距离 = {result['distance']}, 内容 = {result['description']}")