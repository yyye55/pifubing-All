from fastapi import FastAPI, File, UploadFile, Form
import shutil
from pathlib import Path
import zipfile
from langchain_openai import ChatOpenAI
from langchain.schema import HumanMessage, SystemMessage
from lang_faiss import SkinDiseaseSearch
from des import SkinDisease
from test import predict_images
import uvicorn
import logging
from fastapi.middleware.cors import CORSMiddleware

# 初始化 FastAPI 应用
app = FastAPI()

# 配置日志记录
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')

# 配置跨域请求

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# 配置信息
CONFIG = {
    "checkpoint_path": "B3_Transformer.pth",
    "image_size": 300,
    "cpu_threads": 2,
    "model_path": r"D:/学习资料/实验室/皮肤病/skin_disease_2.0/model",
    "index_folder": r"D:\faiss_index",
    "data_file_path": r'D:\学习资料\实验室\皮肤病\skin_disease_2.0\皮肤病材料\皮肤病_丰富版_新增.xlsx',
    "api_key": "REDACTED_LLM_API_KEY",
    "base_url": "https://aistudio.baidu.com/llm/lmapi/v3",
    "model_name": "deepseek-r1"
}


def predict_and_search_images(image_paths):

    try:
        result = predict_images(
            image_paths=image_paths,
            checkpoint_path=CONFIG["checkpoint_path"],
            image_size=CONFIG["image_size"],
            cpu_threads=CONFIG["cpu_threads"]
        )
        logging.info(f"预测类别: {result['final_class']}, 置信度: {result['final_confidence']:.2%}")
        return result
    except Exception as e:
        logging.error(f"图片预测分类失败: {e}")
        return None


def initialize_retrieval():

    try:
        search_engine = SkinDiseaseSearch(CONFIG["model_path"], CONFIG["index_folder"], CONFIG["data_file_path"])
        search = SkinDisease(CONFIG["model_path"], CONFIG["index_folder"], CONFIG["data_file_path"])
        return search_engine, search
    except Exception as e:
        logging.error(f"检索功能初始化失败: {e}")
        return None, None


def retrieve_information(search_engine, question, k=1):

    try:
        search_results = search_engine.search(question, k)
        logging.info(f"检索结果: {search_results}")
        return "\n".join([result["description"] for result in search_results])
    except Exception as e:
        logging.error(f"检索功能失败: {e}")
        return None


def call_llm(chat, system_message, name_info, desc_info, question, predicted_class, confidence, weight=1.0):

    try:
        prediction_info = f"图片预测结果为 {predicted_class}，置信度为 {confidence:.2%}。"

        # 根据权重调整信息组合
        combined_info = []
        if weight < 1.0:
            name_length = int(len(name_info) * weight) if name_info else 0
            desc_length = int(len(desc_info) * (1 - weight)) if desc_info else 0

            combined_info.append("（加权综合检索结果）")
            if name_info:
                combined_info.append(f"名称检索信息（权重{weight:.0%}）：\n{name_info[:name_length]}")
            if desc_info:
                combined_info.append(f"描述检索信息（权重{(1 - weight):.0%}）：\n{desc_info[:desc_length]}")
        else:
            if name_info:
                combined_info.append(f"名称检索信息：\n{name_info}")
            if desc_info:
                combined_info.append(f"描述检索信息：\n{desc_info}")

        combined_system_info = f"{system_message.content}\n{prediction_info}\n" + "\n".join(combined_info)
        combined_system_message = SystemMessage(content=combined_system_info)

        messages = [
            combined_system_message,
            HumanMessage(content=question)
        ]

        full_response = ""
        for chunk in chat.stream(messages):
            if chunk.content:
                full_response += chunk.content
        return full_response
    except Exception as e:
        logging.error(f"大模型调用失败: {e}")
        return None


def handle_file_upload(image: UploadFile):

    image_paths = []
    temp_dir = None
    try:
        if image.filename.endswith('.zip'):
            temp_dir = Path.cwd() / "temp_files"
            temp_dir.mkdir(parents=True, exist_ok=True)

            # 保存并解压ZIP文件
            zip_path = temp_dir / image.filename
            with zip_path.open("wb") as buffer:
                shutil.copyfileobj(image.file, buffer)

            with zipfile.ZipFile(zip_path, 'r') as zip_ref:
                zip_ref.extractall(temp_dir)

            # 删除临时文件夹中的压缩文件
            for file in temp_dir.rglob('*.zip'):
                file.unlink()

            # 收集图片路径
            image_paths = [str(p) for p in temp_dir.rglob('*') if p.is_file()]
            zip_path.unlink()
        else:
            temp_dir = Path.cwd() / "temp_images"
            temp_dir.mkdir(parents=True, exist_ok=True)

            file_path = temp_dir / image.filename
            with file_path.open("wb") as buffer:
                shutil.copyfileobj(image.file, buffer)
            image_paths.append(str(file_path))

        return image_paths, temp_dir
    except Exception as e:
        logging.error(f"文件处理失败: {e}")
        return [], temp_dir


@app.post("/process_image")
async def upload_and_predict(image: UploadFile = File(...)):

    temp_dir = None
    try:
        # 处理文件上传
        image_paths, temp_dir = handle_file_upload(image)
        if not image_paths:
            return {"error": "文件处理失败"}

        # 执行预测
        predict_result = predict_and_search_images(image_paths)
        if not predict_result:
            return {"error": "图片预测失败"}

        return {
            "predicted_class": predict_result["final_class"],
            "confidence": float(predict_result["final_confidence"])
        }
    except Exception as e:
        logging.error(f"预测接口错误: {str(e)}")
        return {"error": str(e)}
    finally:
        # 清理临时文件
        if temp_dir and temp_dir.exists():
            try:
                shutil.rmtree(temp_dir)
                logging.info(f"临时目录已清理: {temp_dir}")
            except Exception as e:
                logging.error(f"清理临时目录失败: {str(e)}")


@app.post("/process_text")
async def generate_response(
        conversation: str = Form(...),
        predicted_class: str = Form(...),
        confidence: float = Form(...)
):

    try:
        # 根据置信度计算权重
        weight = 1.0
        if 70 <= confidence < 85:
            weight = 0.3
            logging.info(f"中等置信度（{confidence}%），启用权重调整，预测结果检索权重: {weight:.0%}，描述检索权重: {(1 - weight):.0%}")
        elif 85 <= confidence <= 100:
            weight = 0.5
            logging.info(f"高置信度（{confidence}%），启用权重调整，预测结果检索权重: {weight:.0%}，描述检索权重: {(1 - weight):.0%}")

        # 初始化检索功能
        search_engine, search = initialize_retrieval()
        if not search_engine or not search:
            return {"error": "检索功能初始化失败"}

        # 执行检索
        name_info = retrieve_information(search_engine, predicted_class)
        desc_info = retrieve_information(search, conversation, k=2)

        # 初始化大模型
        chat = ChatOpenAI(
            api_key=CONFIG["api_key"],
            base_url=CONFIG["base_url"],
            model_name=CONFIG["model_name"],
            streaming=True
        )

        # 生成回复
        system_message = SystemMessage(content="您现在是一名专业的皮肤病医生，需要结合以下信息进行诊断：")
        response = call_llm(
            chat=chat,
            system_message=system_message,
            name_info=name_info,
            desc_info=desc_info,
            question=conversation,
            predicted_class=predicted_class,
            confidence=confidence,
            weight=weight
        )

        if not response:
            return {"error": "生成回复失败"}

        return {
            "generated_text": response,
            "predicted_class": predicted_class,
            "confidence": confidence,
        }
    except Exception as e:
        logging.error(f"生成接口错误: {str(e)}")
        return {"error": str(e)}


if __name__ == "__main__":
    # 运行 FastAPI 应用
    uvicorn.run(app, host='0.0.0.0', port=8888)
