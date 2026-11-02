from fastapi import FastAPI, File, UploadFile, Form
from fastapi.responses import JSONResponse
import shutil
from pathlib import Path
import zipfile
import os
import logging
import uvicorn
from dotenv import load_dotenv

from langchain_openai import ChatOpenAI
from langchain.schema import HumanMessage, SystemMessage
from lang_faiss import SkinDiseaseSearch
from des import SkinDisease
from test import predict_images

from fastapi.middleware.cors import CORSMiddleware

# 读取 .env（本地开发用）；部署时直接配置系统环境变量即可。
load_dotenv()

# 初始化 FastAPI 应用
app = FastAPI(title="皮肤病智能诊断服务", version="1.0.0")

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

BASE_DIR = Path(__file__).resolve().parent

# 所有路径和密钥都从环境变量读取，避免把本机路径、密钥写死在代码里。
CONFIG = {
    # 图片分类模型权重（仓库自带 B3_Transformer.pth）
    "checkpoint_path": os.getenv("MODEL_CHECKPOINT", str(BASE_DIR / "B3_Transformer.pth")),
    "image_size": int(os.getenv("IMAGE_SIZE", "300")),
    "cpu_threads": int(os.getenv("CPU_THREADS", "2")),
    # 文本向量模型目录（需要额外提供，未提供时文本检索功能不可用）
    "model_path": os.getenv("EMBEDDING_MODEL_PATH", ""),
    # FAISS 索引目录（不存在时会根据 Excel 数据自动重建）
    "index_folder": os.getenv("FAISS_INDEX_FOLDER", str(BASE_DIR / "faiss_index")),
    # 皮肤病知识库 Excel（需要额外提供）
    "data_file_path": os.getenv("DATA_FILE_PATH", ""),
    # 大模型配置
    "api_key": os.getenv("LLM_API_KEY", ""),
    "base_url": os.getenv("LLM_BASE_URL", "https://aistudio.baidu.com/llm/lmapi/v3"),
    "model_name": os.getenv("LLM_MODEL", "deepseek-r1"),
    "port": int(os.getenv("PORT", "8001")),
}


def predict_and_search_images(image_paths):
    """调用本地 CNN+Transformer 模型，对图片做皮肤病分类。"""
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


# 检索器加载向量模型和 FAISS 索引比较耗时，进程内只初始化一次。
_RETRIEVAL_CACHE = None
_RETRIEVAL_READY = False


def initialize_retrieval():
    """初始化两个 FAISS 检索器（按疾病名称 / 按描述）。

    缺少向量模型或知识库 Excel 时返回 (None, None)，由调用方降级处理。
    """
    global _RETRIEVAL_CACHE, _RETRIEVAL_READY

    if _RETRIEVAL_READY:
        return _RETRIEVAL_CACHE

    _RETRIEVAL_READY = True
    if not CONFIG["model_path"] or not CONFIG["data_file_path"]:
        logging.warning("未配置 EMBEDDING_MODEL_PATH 或 DATA_FILE_PATH，跳过知识库检索")
        _RETRIEVAL_CACHE = (None, None)
        return _RETRIEVAL_CACHE

    try:
        search_engine = SkinDiseaseSearch(CONFIG["model_path"], CONFIG["index_folder"], CONFIG["data_file_path"])
        search = SkinDisease(CONFIG["model_path"], CONFIG["index_folder"], CONFIG["data_file_path"])
        _RETRIEVAL_CACHE = (search_engine, search)
    except Exception as e:
        logging.error(f"检索功能初始化失败: {e}")
        _RETRIEVAL_CACHE = (None, None)
    return _RETRIEVAL_CACHE


def retrieve_information(search_engine, question, k=1):
    """从 FAISS 索引中检索相关描述；检索器不可用时返回空字符串。"""
    if search_engine is None:
        return ""

    try:
        search_results = search_engine.search(question, k)
        logging.info(f"检索结果: {search_results}")
        return "\n".join([result["description"] for result in search_results])
    except Exception as e:
        logging.error(f"检索功能失败: {e}")
        return ""


def call_llm(chat, system_message, name_info, desc_info, question, predicted_class, confidence, weight=1.0):
    """把检索结果和图片预测结果组合成提示词，调用大模型生成诊断建议。"""
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
    """把上传的图片或压缩包保存到临时目录，返回图片路径列表。"""
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


@app.get("/health")
async def health():
    """健康检查接口，部署时可直接访问确认服务存活。"""
    return {
        "status": "ok",
        "checkpoint_exists": Path(CONFIG["checkpoint_path"]).exists(),
        "retrieval_configured": bool(CONFIG["model_path"] and CONFIG["data_file_path"]),
        "llm_configured": bool(CONFIG["api_key"]),
    }


@app.post("/process_image")
@app.post("/analyze")
async def upload_and_predict(image: UploadFile = File(...), conversation: str = Form("")):
    """图片分类 + 知识库检索接口。

    同时注册 /process_image 和 /analyze：
    前者是本仓库 Python 服务的历史路径，后者是 Go 后端默认调用的路径。
    Go 后端期望返回 prediction / confidence / result_search / desc_search / weight，
    因此这里在分类之外补充检索字段；未配置知识库时检索字段为空。
    """
    temp_dir = None
    try:
        # 处理文件上传
        image_paths, temp_dir = handle_file_upload(image)
        if not image_paths:
            return JSONResponse(status_code=400, content={"error": "文件处理失败"})

        # 执行预测
        predict_result = predict_and_search_images(image_paths)
        if not predict_result:
            return JSONResponse(status_code=500, content={"error": "图片预测失败"})

        predicted_class = predict_result["final_class"]
        confidence = float(predict_result["final_confidence"])

        # 根据置信度决定“名称检索”和“描述检索”的权重
        weight = 1.0
        if 0.70 <= confidence < 0.85:
            weight = 0.3
        elif 0.85 <= confidence <= 1.0:
            weight = 0.5

        name_info = ""
        desc_info = ""
        search_engine, search = initialize_retrieval()
        if search_engine is not None and search is not None:
            name_info = retrieve_information(search_engine, predicted_class)
            if conversation:
                desc_info = retrieve_information(search, conversation, k=2)

        # 同时保留 predicted_class 别名，兼容旧调用方。
        return {
            "predicted_class": predicted_class,
            "prediction": predicted_class,
            "confidence": confidence,
            "result_search": name_info,
            "desc_search": desc_info,
            "weight": f"{weight}",
        }
    except Exception as e:
        logging.error(f"预测接口错误: {str(e)}")
        return JSONResponse(status_code=500, content={"error": str(e)})
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
    """文字问诊接口：检索知识库 + 调用大模型生成回答。

    未配置知识库时仍可调用大模型（只基于用户描述），保证服务可用。
    """
    try:
        # 根据置信度计算权重
        weight = 1.0
        if 70 <= confidence < 85:
            weight = 0.3
            logging.info(f"中等置信度（{confidence}%），启用权重调整，预测结果检索权重: {weight:.0%}，描述检索权重: {(1 - weight):.0%}")
        elif 85 <= confidence <= 100:
            weight = 0.5
            logging.info(f"高置信度（{confidence}%），启用权重调整，预测结果检索权重: {weight:.0%}，描述检索权重: {(1 - weight):.0%}")

        # 初始化检索功能；缺少数据时降级为“不检索”
        search_engine, search = initialize_retrieval()

        # 执行检索
        name_info = retrieve_information(search_engine, predicted_class)
        desc_info = retrieve_information(search, conversation, k=2)

        if not CONFIG["api_key"]:
            return JSONResponse(status_code=500, content={"error": "未配置 LLM_API_KEY，无法调用大模型"})

        # 初始化大模型
        chat = ChatOpenAI(
            api_key=CONFIG["api_key"],
            base_url=CONFIG["base_url"],
            model=CONFIG["model_name"],
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
            return JSONResponse(status_code=500, content={"error": "生成回复失败"})

        return {
            "generated_text": response,
            "predicted_class": predicted_class,
            "confidence": confidence,
        }
    except Exception as e:
        logging.error(f"生成接口错误: {str(e)}")
        return JSONResponse(status_code=500, content={"error": str(e)})


if __name__ == "__main__":
    # 运行 FastAPI 应用；端口可通过 PORT 环境变量覆盖。
    uvicorn.run(app, host='0.0.0.0', port=CONFIG["port"])
