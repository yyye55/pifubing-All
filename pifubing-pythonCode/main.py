from fastapi import FastAPI, File, UploadFile, Form
from fastapi.responses import JSONResponse, StreamingResponse
import json
import shutil
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path
import zipfile
import os
import re
import logging
import uvicorn
from dotenv import load_dotenv

from langchain_openai import ChatOpenAI
from langchain.schema import HumanMessage, SystemMessage
from lang_faiss import SkinDiseaseSearch
from des import SkinDisease
from test import predict_images, check_image_quality
from vision_structure import describe_image_structure

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


def build_llm_messages(system_message, name_info, desc_info, question, predicted_class, confidence, weight=1.0):
    """把检索结果和图片预测结果组合成提示词，供普通调用和流式调用共用。"""
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

    return [
        combined_system_message,
        HumanMessage(content=question)
    ]


def call_llm(chat, system_message, name_info, desc_info, question, predicted_class, confidence, weight=1.0):
    """非流式调用：收集完整回复后返回，保持原有 /process_text 行为。"""
    try:
        messages = build_llm_messages(
            system_message, name_info, desc_info, question, predicted_class, confidence, weight
        )

        full_response = ""
        for chunk in chat.stream(messages):
            if chunk.content:
                full_response += chunk.content
        return full_response
    except Exception as e:
        logging.error(f"大模型调用失败: {e}")
        return None


def stream_llm(chat, system_message, name_info, desc_info, question, predicted_class, confidence, weight=1.0):
    """流式调用：逐块产出 ("thoughts", 推理内容) 或 ("message", 回答内容)。"""
    messages = build_llm_messages(
        system_message, name_info, desc_info, question, predicted_class, confidence, weight
    )

    for chunk in chat.stream(messages):
        # DeepSeek-R1 等推理模型可能把思考过程放在 reasoning_content 中
        extra = getattr(chunk, "additional_kwargs", None) or {}
        reasoning = extra.get("reasoning_content") or ""
        if reasoning:
            yield "thoughts", reasoning

        if chunk.content:
            yield "message", chunk.content


FOLLOWUP_SYSTEM_PROMPT = (
    "你是一名皮肤科问诊助手。请根据用户的问题和医生的回答，"
    "生成最多3个用户接下来最可能继续追问的问题。\n"
    "要求：\n"
    "1. 每个问题不超过20个汉字，口语化；\n"
    "2. 必须与当前问答直接相关，不要编造诊断、药物剂量或治疗承诺；\n"
    "3. 只输出 JSON 数组，例如 [\"问题1\", \"问题2\"]，"
    "不要输出解释、编号或代码块。"
)


def generate_followups(question, answer):
    """根据本轮问答生成最多3个追问；任何失败都返回空列表，不影响主回答。"""
    if not question or not answer or not answer.strip():
        return []

    try:
        chat = ChatOpenAI(
            api_key=CONFIG["api_key"],
            base_url=CONFIG["base_url"],
            model=CONFIG["model_name"],
            streaming=False,
            max_tokens=200,
            timeout=12,
        )
        messages = [
            SystemMessage(content=FOLLOWUP_SYSTEM_PROMPT),
            HumanMessage(content=f"用户问题：{question}\n医生回答：{answer[:2000]}"),
        ]
        response = chat.invoke(messages)
        raw = response.content if isinstance(response.content, str) else str(response.content)
        raw = raw.strip()

        # 兼容模型偶尔返回 ```json 代码块或前后带解释文字的情况。
        match = re.search(r"\[[\s\S]*\]", raw)
        items = []
        if match:
            try:
                parsed = json.loads(match.group(0))
                if isinstance(parsed, list):
                    items = parsed
            except json.JSONDecodeError:
                items = []
        if not items:
            items = [line.strip(" -•*\t") for line in raw.splitlines() if line.strip()]

        suggestions = []
        for item in items:
            if not isinstance(item, str):
                continue
            text = re.sub(r"^[\d\s\.\、\)\-]+", "", item).strip(" \"'“”")
            if not text or len(text) > 40 or text in suggestions:
                continue
            suggestions.append(text)
            if len(suggestions) == 3:
                break
        return suggestions
    except Exception as e:
        logging.warning(f"生成追问失败: {e}")
        return []


HOT_QUESTIONS_SYSTEM_PROMPT = (
    "你是一名皮肤科问诊平台的助手。请生成若干近期用户最常咨询、最热门的皮肤科问题。\n"
    "要求：\n"
    "1. 覆盖不同的常见皮肤困扰（如痤疮、湿疹、荨麻疹、脚气、脱发、晒伤等），不要重复；\n"
    "2. 每个问题包含 label（4-8个汉字的短标题）和 text（20-45个汉字、口语化的完整问题）；\n"
    "3. 不要包含诊断结论、药物剂量或治疗承诺；\n"
    "4. 只输出 JSON 数组，例如 "
    "[{\"label\":\"痘痘反复\",\"text\":\"脸上痘痘总是反反复复，平时应该怎么护理？\"}]，"
    "不要输出解释、编号或代码块。"
)


def generate_hot_questions(max_items=4):
    """让大模型生成首页展示的热门皮肤科问题。

    任何失败都返回空列表，由前端回退到内置问题，不影响首页正常使用。
    """
    if max_items <= 0:
        return []

    try:
        chat = ChatOpenAI(
            api_key=CONFIG["api_key"],
            base_url=CONFIG["base_url"],
            model=CONFIG["model_name"],
            streaming=False,
            # 一次生成一整池问题，token 和时间都要比单个追问留得更多。
            max_tokens=1200,
            timeout=40,
        )
        messages = [
            SystemMessage(content=HOT_QUESTIONS_SYSTEM_PROMPT),
            HumanMessage(content=f"请生成 {max_items} 个热门皮肤科问题。"),
        ]
        response = chat.invoke(messages)
        raw = response.content if isinstance(response.content, str) else str(response.content)
        raw = raw.strip()

        # 兼容模型偶尔返回 ```json 代码块或前后带解释文字的情况。
        match = re.search(r"\[[\s\S]*\]", raw)
        items = []
        if match:
            try:
                parsed = json.loads(match.group(0))
                if isinstance(parsed, list):
                    items = parsed
            except json.JSONDecodeError:
                items = []

        questions = []
        seen = set()
        for item in items:
            if isinstance(item, str):
                label, text = "", item.strip()
            elif isinstance(item, dict):
                label = str(item.get("label", "")).strip()
                text = str(item.get("text", "")).strip()
            else:
                continue

            label = re.sub(r"^[\d\s\.\、\)\-\#\*]+", "", label).strip(" \"'“”")
            text = re.sub(r"^[\d\s\.\、\)\-\#\*]+", "", text).strip(" \"'“”")
            # 完整问题为空、过长或重复的条目直接丢弃，避免前端渲染出无效按钮。
            if not text or len(text) > 60 or text in seen:
                continue
            if not label or len(label) > 12:
                # 模型没给短标题时，用问题开头兜底，保证按钮上有可读文字。
                label = text[:8]
            seen.add(text)
            questions.append({"label": label, "text": text})
            if len(questions) == max_items:
                break

        return questions
    except Exception as e:
        logging.warning(f"生成热门问题失败: {e}")
        return []


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

        # 先做质量检测：任一张合格即放行，zip 多图里一张模糊不拖累整包。
        quality_results = [check_image_quality(path) for path in image_paths]
        quality_ok = any(item["ok"] for item in quality_results)
        quality_reasons = []
        if not quality_ok:
            for item in quality_results:
                for reason in item["reasons"]:
                    if reason not in quality_reasons:
                        quality_reasons.append(reason)
            logging.info(f"图片质量不合格: {quality_reasons}")
            return {
                "predicted_class": None,
                "prediction": None,
                "confidence": 0.0,
                "candidates": [],
                "result_search": "",
                "desc_search": "",
                "weight": "1.0",
                "quality_ok": False,
                "quality_reasons": quality_reasons,
                "structure": {},
                "structure_summary": "",
            }

        # 本地分类和视觉结构并行：视觉接口慢/失败时只降级结构信息，
        # 不影响本地分类结果和后续问诊。
        with ThreadPoolExecutor(max_workers=2) as pool:
            predict_future = pool.submit(predict_and_search_images, image_paths)
            structure_future = pool.submit(describe_image_structure, image_paths[0])
            predict_result = predict_future.result()
            structure = structure_future.result()
        if not predict_result:
            return JSONResponse(status_code=500, content={"error": "图片预测失败"})

        predicted_class = predict_result["final_class"]
        confidence = float(predict_result["final_confidence"])
        candidates = predict_result.get("candidates", [])

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
            # top-3 每个候选都检索一份资料，供大模型在候选之间做判断，
            # 而不是只拿到 top-1 一个病名。
            info_parts = []
            for candidate in candidates[:3]:
                info = retrieve_information(search_engine, candidate["prediction"])
                if info:
                    info_parts.append(
                        f"【{candidate['prediction']}（{candidate['confidence']:.2%}）】\n{info}"
                    )
            name_info = "\n\n".join(info_parts)
            if conversation:
                desc_info = retrieve_information(search, conversation, k=2)

        # 同时保留 predicted_class 别名，兼容旧调用方。
        return {
            "predicted_class": predicted_class,
            "prediction": predicted_class,
            "confidence": confidence,
            "candidates": candidates,
            "result_search": name_info,
            "desc_search": desc_info,
            "weight": f"{weight}",
            "quality_ok": True,
            "quality_reasons": [],
            "structure": structure,
            "structure_summary": structure.get("summary", "") if isinstance(structure, dict) else "",
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
    """文字问诊接口：检索知识库并返回分类结果与检索内容。

    比赛演示阶段暂不调用大模型：仅做知识库检索，直接返回分类结果与检索内容，
    避免未配置 LLM_API_KEY 时接口直接返回 500。
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

        # ===== 大模型调用：未配置 API Key 或调用失败时降级为仅返回检索结果 =====
        generated_text = ""
        if not CONFIG["api_key"]:
            logging.warning("未配置 LLM_API_KEY，仅返回知识库检索结果")
        else:
            try:
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

                if response:
                    generated_text = response
                else:
                    logging.warning("大模型未返回内容，降级为仅返回知识库检索结果")
            except Exception as e:
                logging.error(f"大模型调用异常，降级为仅返回知识库检索结果: {e}")

        return {
            "predicted_class": predicted_class,
            "result_search": name_info,
            "desc_search": desc_info,
            "generated_text": generated_text,
        }
    except Exception as e:
        logging.error(f"生成接口错误: {str(e)}")
        return JSONResponse(status_code=500, content={"error": str(e)})


def sse_payload(payload):
    """把字典编码成一行 SSE data，确保 JSON 内的换行不会破坏事件边界。"""
    return f"data: {json.dumps(payload, ensure_ascii=False)}\n\n"


@app.post("/process_text_stream")
async def generate_response_stream(
        conversation: str = Form(...),
        predicted_class: str = Form(""),
        confidence: float = Form(0.0)
):
    """文字问诊流式接口：先做知识库检索，再把大模型回答逐块推给 Go 后端。

    返回格式为 SSE：
      data: {"content": "..."}
      data: {"thoughts": "..."}
      data: [DONE]
    Go 后端会把这些事件再转发给浏览器。
    """
    try:
        # 根据置信度计算权重；纯文字问诊通常没有图片置信度，默认 weight=1.0
        weight = 1.0
        if 70 <= confidence < 85:
            weight = 0.3
        elif 85 <= confidence <= 100:
            weight = 0.5

        search_engine, search = initialize_retrieval()
        name_info = retrieve_information(search_engine, predicted_class)
        desc_info = retrieve_information(search, conversation, k=2)

        headers = {
            "Cache-Control": "no-cache",
            "Connection": "keep-alive",
            "X-Accel-Buffering": "no",
        }

        # 没配置 Key 时也要用 SSE 返回，避免前端一直停在“思考中”
        if not CONFIG["api_key"]:
            logging.warning("未配置 LLM_API_KEY，流式接口返回提示文本")

            def fallback_stream():
                yield sse_payload({"content": "当前未配置大模型服务，暂时无法生成回答。"})
                yield "data: [DONE]\n\n"

            return StreamingResponse(
                fallback_stream(),
                media_type="text/event-stream",
                headers=headers
            )

        chat = ChatOpenAI(
            api_key=CONFIG["api_key"],
            base_url=CONFIG["base_url"],
            model=CONFIG["model_name"],
            streaming=True
        )
        system_message = SystemMessage(content="您现在是一名专业的皮肤病医生，需要结合以下信息进行诊断：")

        def event_stream():
            answer_parts = []
            try:
                for kind, text in stream_llm(
                    chat=chat,
                    system_message=system_message,
                    name_info=name_info,
                    desc_info=desc_info,
                    question=conversation,
                    predicted_class=predicted_class,
                    confidence=confidence,
                    weight=weight
                ):
                    if kind == "thoughts":
                        yield sse_payload({"thoughts": text})
                    else:
                        answer_parts.append(text)
                        yield sse_payload({"content": text})

                # 回答完整输出后，再生成最多3个相关追问，供前端做成可点击的快捷问题。
                suggestions = generate_followups(conversation, "".join(answer_parts))
                if suggestions:
                    yield sse_payload({"suggestions": suggestions})
            except Exception as e:
                logging.error(f"流式大模型调用失败: {e}")
                yield sse_payload({"error": "大模型调用失败，请稍后再试。"})
            # 正常结束或发生异常时补一个结束标记；客户端主动断开（GeneratorExit）
            # 时不应再向已关闭的连接写数据，所以不能放在 finally 里。
            yield "data: [DONE]\n\n"

        return StreamingResponse(
            event_stream(),
            media_type="text/event-stream",
            headers=headers
        )
    except Exception as e:
        logging.error(f"流式生成接口错误: {str(e)}")
        return JSONResponse(status_code=500, content={"error": str(e)})


@app.post("/generate_followups")
async def generate_followups_endpoint(
        question: str = Form(...),
        answer: str = Form(...)
):
    """供图片问答等非 Python 主流程复用：根据问答内容生成最多3个追问。"""
    return {"suggestions": generate_followups(question, answer)}


@app.get("/generate_hot_questions")
def generate_hot_questions_endpoint(count: int = 4):
    """供 Go 后端获取首页热门皮肤科问题。

    失败时返回空数组，前端会继续使用内置的兜底问题。
    这里用普通 def 而不是 async def，让 FastAPI 把阻塞的模型调用放到线程池执行。
    """
    # 后端一次会请求一整池（默认 8 个），上限放宽到 20。
    count = max(1, min(count, 20))
    return {"questions": generate_hot_questions(count)}


if __name__ == "__main__":
    # 运行 FastAPI 应用；端口可通过 PORT 环境变量覆盖。
    uvicorn.run(app, host='0.0.0.0', port=CONFIG["port"])
