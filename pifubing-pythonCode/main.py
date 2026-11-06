from fastapi import FastAPI, File, UploadFile, Form, Header, HTTPException
from fastapi.responses import JSONResponse, StreamingResponse
from pydantic import BaseModel
import json
import io
import shutil
import threading
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path
import zipfile
import os
import re
import logging
import uuid
import uvicorn
from dotenv import load_dotenv

from langchain_openai import ChatOpenAI
from langchain.schema import HumanMessage, SystemMessage
from lang_faiss import SkinDiseaseSearch
from des import SkinDisease
from test import predict_images, check_image_quality
from vision_structure import describe_image_structure
from memory_service import memory_service
# 提示词集中在 prompts.py，文字问诊、图片问诊和追问共用同一套风格与安全边界。
from prompts import (
    PROMPT_STYLE_VERSION,
    SKIN_DOCTOR_SYSTEM_PROMPT,
    FOLLOWUP_SYSTEM_PROMPT,
    OFF_TOPIC_REFUSAL,
)

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


def _env_float(name, default):
    """安全读取浮点环境变量；写错时回退默认值，避免服务启动失败。"""
    try:
        value = float(os.getenv(name, str(default)))
    except (TypeError, ValueError):
        return default
    return min(max(value, 0.0), 1.0)


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
    "base_url": os.getenv("LLM_BASE_URL", "https://open.bigmodel.cn/api/paas/v4"),
    "model_name": os.getenv("LLM_MODEL", "glm-4-flash"),
    # 医生回答默认偏低温度，减少措辞漂移；可用环境变量调整，无需改代码。
    "temperature": _env_float("LLM_TEMPERATURE", 0.3),
    "top_p": _env_float("LLM_TOP_P", 0.9),
    "port": int(os.getenv("PORT", "8001")),
    # 语音识别（浏览器录音转文字）：模型文件首次使用时下载到该目录
    "asr_model": os.getenv("ASR_MODEL", "small"),
    "asr_model_dir": os.getenv("ASR_MODEL_DIR", str(BASE_DIR / "models")),
    "asr_language": os.getenv("ASR_LANGUAGE", "zh"),
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


# 明确与皮肤无关的请求才做确定性拦截；只要问题里出现皮肤关键词，
# 就交给大模型按上下文判断，避免把“天气热导致湿疹加重”这类问题误判成闲聊。
_SKIN_KEYWORDS = (
    "皮肤", "皮疹", "红疹", "湿疹", "荨麻疹", "痤疮", "痘痘", "疹", "痘",
    "癣", "脱发", "头皮", "指甲", "瘙痒", "痒", "过敏", "水疱", "伤口", "药膏", "用药",
)
_OFF_TOPIC_PATTERNS = (
    re.compile(r"(?:帮我|请|给我|能不能|可以)?\s*(?:写|作|创作).{0,8}(?:诗|诗歌|故事|小说|作文)"),
    re.compile(
        r"(?:帮我|请|给我|能不能|可以)?\s*(?:写|实现|改).{0,12}"
        r"(?:代码|程序|脚本|python|java|go语言|c\+\+|快速排序|算法)"
    ),
    re.compile(r"(?:今天|明天|现在|最近).{0,10}(?:天气|气温|下雨|温度)"),
    re.compile(r"(?:推荐|介绍|找).{0,8}(?:电影|电视剧|综艺|动漫|歌曲|音乐)"),
    re.compile(r"(?:讲|说).{0,4}(?:笑话|段子)"),
)


def is_clearly_off_topic(question):
    """判断是否为“写诗/写代码/查天气/推荐影视”等明确无关请求。

    这是提示词之外的确定性兜底，只覆盖非常明确的模式；
    带皮肤关键词的问题一律放行，交给模型结合上下文回答。
    """
    text = (question or "").strip().lower()
    if not text:
        return False
    if any(word in text for word in _SKIN_KEYWORDS):
        return False
    return any(pattern.search(text) for pattern in _OFF_TOPIC_PATTERNS)


def build_llm_messages(system_message, name_info, desc_info, question, predicted_class, confidence,
                       weight=1.0, memory_context=""):
    """把检索结果、图片预测结果和历史上下文组合成提示词，供普通调用和流式调用共用。"""
    # 纯文字问诊没有图片预测结果，不拼这一段，避免把“置信度 0.00%”写进提示词。
    # 图片问诊也只给病名、不把百分比交给大模型，防止回答里出现模型指标。
    prediction_info = ""
    if predicted_class and confidence > 0:
        prediction_info = (
            f"图片模型倾向的皮肤病是 {predicted_class}。"
            "【图片问诊硬性要求】本次回答必须逐字以“从照片看，可能是”开头，"
            "不能写成“从描述看”“根据图片看”“看起来”等变体；"
            "这句话放在最前面，不要加任何开场白。"
            "这条规则优先于“信息不足时先说无法判断”的风格要求："
            "即使信息不足，也必须先写“从照片看，可能是XX”，再追问或建议就医。"
            "如果还有其他可能，紧接着补一句“也可能是YY”；"
            "不要用“根据您提供的信息”“感谢您的信任”之类的开场白；"
            "不要提及置信度、概率或百分比。"
        )
        if confidence < 80:
            prediction_info += (
                "模型对这张照片把握不大，第一句请写成“从照片看，可能是XX，但不太确定”，"
                "并建议就医确认。"
            )

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

    combined_system_info = system_message.content
    if prediction_info:
        combined_system_info += f"\n{prediction_info}"
    if combined_info:
        combined_system_info += "\n" + "\n".join(combined_info)
    # 历史上下文由 Go 后端整理好后单独传入，不拼进 question。
    # 这样 FAISS 知识库检索仍然只用当前问题，不会被几百字历史带偏。
    if memory_context and memory_context.strip():
        # 用“用户档案”而不是“仅供参考”的措辞：GLM-4-Flash 对弱指令很容易忽略，
        # 但这份档案只包含用户本人说过的事实，回答时应当作为已知信息使用。
        combined_system_info += (
            "\n【已知用户档案与最近对话】以下内容来自同一对话的最近问答和用户过去主动说过的信息。"
            "回答时如果与当前问题相关必须使用，但不要逐条复述，"
            "也不要把以前的判断当成当前诊断。"
            "如果当前问题涉及既往病史、过敏史或正在使用的药物，请优先使用这些信息，"
            "必要时用“您之前提到……”自然说明。"
            "如果档案里明确写有过敏原，而当前问题涉及用药，回答必须点名该过敏原"
            "（例如“您之前提到对青霉素过敏”），不能只写笼统的“注意过敏史”。\n"
            + memory_context.strip()
        )
    combined_system_message = SystemMessage(content=combined_system_info)

    return [
        combined_system_message,
        HumanMessage(content=question)
    ]


def call_llm(chat, system_message, name_info, desc_info, question, predicted_class, confidence,
             weight=1.0, memory_context=""):
    """非流式调用：收集完整回复后返回，保持原有 /process_text 行为。"""
    # 明确无关的请求直接返回固定话术，不调用大模型，避免浪费 token 和越界回答。
    if is_clearly_off_topic(question):
        return OFF_TOPIC_REFUSAL
    try:
        messages = build_llm_messages(
            system_message, name_info, desc_info, question, predicted_class, confidence,
            weight, memory_context
        )

        full_response = ""
        for chunk in chat.stream(messages):
            if chunk.content:
                full_response += chunk.content
        return full_response
    except Exception as e:
        logging.error(f"大模型调用失败: {e}")
        return None


def stream_llm(chat, system_message, name_info, desc_info, question, predicted_class, confidence,
               weight=1.0, memory_context=""):
    """流式调用：逐块产出 ("thoughts", 推理内容) 或 ("message", 回答内容)。"""
    # 与非流式保持一致：明确无关的请求只返回固定话术，不调用大模型。
    if is_clearly_off_topic(question):
        yield "message", OFF_TOPIC_REFUSAL
        return

    messages = build_llm_messages(
        system_message, name_info, desc_info, question, predicted_class, confidence,
        weight, memory_context
    )

    for chunk in chat.stream(messages):
        # DeepSeek-R1 等推理模型可能把思考过程放在 reasoning_content 中
        extra = getattr(chunk, "additional_kwargs", None) or {}
        reasoning = extra.get("reasoning_content") or ""
        if reasoning:
            yield "thoughts", reasoning

        if chunk.content:
            yield "message", chunk.content


# 提示词已集中到 prompts.py：
# - SKIN_DOCTOR_SYSTEM_PROMPT：文字/图片问诊共用的安全边界与正式回答风格
# - FOLLOWUP_SYSTEM_PROMPT：追问生成，仍保持纯 JSON 输出
# 这里只保留 import，避免两处提示词逐渐不一致。


def generate_followups(question, answer):
    """根据本轮问答生成最多3个追问；任何失败都返回空列表，不影响主回答。"""
    if not question or not answer or not answer.strip():
        return []
    # 固定拒绝话术不需要生成追问，省一次模型调用，也避免把无关话题带回来。
    if answer.strip().startswith(OFF_TOPIC_REFUSAL):
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
        else:
            # 只有模型完全没按 JSON 返回时才按行兜底。
            # 明确的空数组 [] 表示“不生成追问”，不能把字符串 "[]" 当成一个问题。
            items = [line.strip(" -•*\t") for line in raw.splitlines() if line.strip()]

        suggestions = []
        for item in items:
            if not isinstance(item, str):
                continue
            text = re.sub(r"^[\d\s\.\、\)\-]+", "", item).strip(" \"'“”")
            if (not text or text in ("[]", "[ ]") or len(text) > 40
                    or text in suggestions):
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


# 个人中心“AI 健康档案”的系统提示词。
# 只允许模型归纳已有信息，明确禁止编造诊断、药物剂量和治疗承诺。
HEALTH_ARCHIVE_SYSTEM_PROMPT = (
    "你是一名皮肤科健康档案助理，负责把用户填写的个人皮肤情况和历史 AI 问诊记录，"
    "整理成一份详实、可长期跟踪的个人皮肤健康档案。\n"
    "请使用 Markdown，按下面的结构输出；没有信息的小节写“暂无相关记录”，但不要省略小节：\n"
    "## 一、档案概览\n"
    "用 3-5 句话概括用户的基本情况、目前最主要的皮肤困扰，以及本档案的数据来源"
    "（个人自述和最近若干条问诊记录），并写明个人资料中提供的建档日期，让用户一眼看懂档案重点。\n"
    "## 二、基本情况\n"
    "列出姓名、年龄、性别、建档日期等基础信息；如果年龄能推断所处阶段（如青少年、成年、中年），"
    "可简要说明该阶段皮肤护理需要关注的重点。\n"
    "## 三、皮肤问题梳理\n"
    "只列用户实际出现过的症状或皮损，每条尽量写清：问题或表现、出现部位、持续时间或发作频次、"
    "可能的诱因、目前采用的处理方式。多条记录指向同一问题时合并归纳，并注明“多次提及”，"
    "体现问题的反复情况；预防、护肤、用药等科普类咨询不算皮肤问题，放到问诊记录摘要中。\n"
    "## 四、症状特点与诱因分析\n"
    "结合记录归纳症状的共同特点（如瘙痒、红斑、脱屑、干燥、渗出、色素改变等）"
    "以及可能的加重或缓解因素。明确区分“用户自述”与“AI 既往建议”，"
    "不确定的内容用“可能与……有关”表述，不下确定性诊断。\n"
    "## 五、过敏史与注意事项\n"
    "列出已知过敏原、药物过敏和食物过敏，并说明需要避免的接触物、护肤品成分或生活习惯。\n"
    "## 六、日常护理建议\n"
    "按清洁、保湿、防晒、衣物、居住环境、作息与饮食分组，结合用户的具体情况给出可执行的做法"
    "（例如洗澡水温与时长、保湿频率、防晒方式、衣物材质），不要只写通用套话。\n"
    "## 七、复发预警信号\n"
    "列出需要留意或应及时复诊的表现，例如皮损范围明显扩大、渗液、持续疼痛、伴发热等。\n"
    "## 八、随访与就医建议\n"
    "给出复诊时机、建议就诊科室，以及就诊前可以准备的资料；提醒 AI 建议不能替代线下诊断。\n"
    "## 九、问诊记录摘要\n"
    "用列表按主题概括最近问诊的关键内容，每条标注对应的皮肤问题和 AI 的主要建议，"
    "便于用户回顾。\n"
    "## 十、数据说明与免责声明\n"
    "说明本档案由 AI 根据用户填写信息和历史问诊自动整理，可能存在遗漏，仅供健康管理参考。\n"
    "写作约束：\n"
    "1. 只依据给出的信息整理，不得编造没有出现的症状、诊断、药物、检查结果或就诊经历；\n"
    "2. 不给出具体药物剂量、处方或治疗承诺，涉及用药时只写“遵医嘱使用”；\n"
    "3. 内容要具体、贴合用户描述，避免模板化套话，总长度控制在 800-1200 字；\n"
    "4. 历史问诊中与皮肤、毛发、指甲无关的内容直接忽略，不要写入档案的任何小节；\n"
    "5. 只输出档案正文，不要输出解释、前言或代码块。"
)


def generate_health_archive(profile, history):
    """根据个人皮肤情况和问诊记录生成健康档案。

    profile 为用户填写的姓名、年龄、性别、病史、过敏史等；
    history 为最近若干条问诊的问答摘要。
    任何失败都返回空字符串，由 Go 层向前端返回友好错误。
    """
    profile = (profile or "").strip()
    history = (history or "").strip()
    if not profile and not history:
        return ""

    try:
        chat = ChatOpenAI(
            api_key=CONFIG["api_key"],
            base_url=CONFIG["base_url"],
            model=CONFIG["model_name"],
            streaming=False,
            # 档案扩充到 800-1200 字后需要更多输出额度，避免正文被截断。
            max_tokens=2500,
            # 汇总多条问诊比单个回答需要更多时间，但也不能无限等待。
            timeout=75,
        )
        messages = [
            SystemMessage(content=HEALTH_ARCHIVE_SYSTEM_PROMPT),
            HumanMessage(
                content=(
                    f"个人皮肤情况：\n{profile or '暂无'}\n\n"
                    # 限制输入长度，避免历史记录很多时请求体过大、模型耗时过长。
                    # 记录按时间先后拼接，保留末尾内容即保留最近的问诊。
                    f"历史问诊记录：\n{history[-6000:] or '暂无'}"
                )
            ),
        ]
        response = chat.invoke(messages)
        raw = response.content if isinstance(response.content, str) else str(response.content)
        raw = raw.strip()

        # 兼容模型偶尔返回 ```markdown 代码块的情况。
        raw = re.sub(r"^```(?:markdown)?\s*", "", raw, flags=re.IGNORECASE)
        raw = re.sub(r"\s*```$", "", raw)
        return raw.strip()
    except Exception as e:
        logging.warning(f"生成健康档案失败: {e}")
        return ""


def handle_file_upload(image: UploadFile):
    """把上传的图片或压缩包保存到临时目录，返回图片路径列表。"""
    image_paths = []
    temp_dir = None
    try:
        # 每次请求使用独立临时目录，避免并发请求互相污染，也避免旧图片被重复收集。
        temp_dir = Path.cwd() / "temp_files" / uuid.uuid4().hex
        temp_dir.mkdir(parents=True, exist_ok=True)

        if image.filename.endswith('.zip'):
            # 保存并解压ZIP文件
            zip_path = temp_dir / image.filename
            with zip_path.open("wb") as buffer:
                shutil.copyfileobj(image.file, buffer)

            with zipfile.ZipFile(zip_path, 'r') as zip_ref:
                zip_ref.extractall(temp_dir)

            # 收集图片路径，压缩包本身不参与预测
            image_paths = [
                str(p) for p in temp_dir.rglob('*')
                if p.is_file() and p.suffix.lower() != '.zip'
            ]
        else:
            file_path = temp_dir / image.filename
            with file_path.open("wb") as buffer:
                shutil.copyfileobj(image.file, buffer)
            image_paths.append(str(file_path))

        return image_paths, temp_dir
    except Exception as e:
        logging.error(f"文件处理失败: {e}")
        return [], temp_dir


# Whisper 模型加载比较慢（首次还要下载模型），进程内只加载一次。
_ASR_MODEL = None
_ASR_MODEL_LOCK = threading.Lock()

# 给 Whisper 一段皮肤科常见词提示，能明显提升“荨麻疹、银屑病”等专业词的识别率。
ASR_INITIAL_PROMPT = (
    "以下是皮肤科问诊的普通话录音，可能提到瘙痒、红疹、湿疹、荨麻疹、"
    "痤疮、银屑病、皮炎、真菌感染、脱毛、脱发、水疱、脱屑、过敏等词语。"
)


def get_asr_model():
    """按需加载 faster-whisper 模型。

    用普通函数而不是全局初始化：没有语音请求时不占内存，也不会拖慢服务启动。
    国内网络直连 HuggingFace 往往失败，默认走 hf-mirror 镜像（可用 HF_ENDPOINT 覆盖）。
    """
    global _ASR_MODEL

    if _ASR_MODEL is not None:
        return _ASR_MODEL

    with _ASR_MODEL_LOCK:
        if _ASR_MODEL is None:
            os.environ.setdefault("HF_ENDPOINT", "https://hf-mirror.com")
            # Windows 未开启开发者模式时不支持缓存软链接，关闭对应的警告噪音。
            os.environ.setdefault("HF_HUB_DISABLE_SYMLINKS_WARNING", "1")
            # 延迟导入：即使没装 faster-whisper，图片和文字问诊功能仍能正常启动。
            from faster_whisper import WhisperModel

            model_dir = Path(CONFIG["asr_model_dir"])
            model_dir.mkdir(parents=True, exist_ok=True)
            logging.info(f"正在加载语音识别模型 {CONFIG['asr_model']}，首次使用需要下载，请稍候...")
            _ASR_MODEL = WhisperModel(
                CONFIG["asr_model"],
                device="cpu",
                compute_type="int8",
                download_root=str(model_dir),
                cpu_threads=4,
            )
            logging.info("语音识别模型加载完成")
    return _ASR_MODEL


@app.on_event("startup")
def preload_asr_model():
    """服务启动后在后台预加载 Whisper，避免第一次语音输入要等 20 多秒。"""
    def _load():
        try:
            get_asr_model()
        except Exception as e:
            # 预加载失败不影响服务启动，第一次语音请求时会再尝试。
            logging.warning(f"语音识别模型预加载失败，首次语音请求时会重试: {e}")

    threading.Thread(target=_load, daemon=True).start()


@app.on_event("startup")
def preload_memory_service():
    """服务启动后在后台初始化长期记忆，避免第一个问诊请求多等约 1 秒。

    初始化失败只会打日志，图片/文字问诊仍按无记忆模式正常运行。
    """
    if memory_service.is_enabled():
        threading.Thread(target=memory_service.warmup, daemon=True).start()


@app.get("/health")
async def health():
    """健康检查接口，部署时可直接访问确认服务存活。"""
    return {
        "status": "ok",
        "checkpoint_exists": Path(CONFIG["checkpoint_path"]).exists(),
        "retrieval_configured": bool(CONFIG["model_path"] and CONFIG["data_file_path"]),
        "llm_configured": bool(CONFIG["api_key"]),
        # 便于部署时确认记忆功能是否开启；不含任何用户数据。
        "memory_enabled": memory_service.is_enabled(),
        # 便于确认当前生效的提示词版本，不包含任何用户数据。
        "prompt_style_version": PROMPT_STYLE_VERSION,
    }


@app.post("/asr")
def asr_endpoint(file: UploadFile = File(...)):
    """浏览器录音转文字：前端用 MediaRecorder 录 webm/ogg，这里直接交给 Whisper 解码。

    用普通 def 让 FastAPI 把阻塞的转写放到线程池，避免卡住其他接口。
    """
    try:
        audio = file.file.read()
        if not audio:
            return JSONResponse(status_code=400, content={"error": "音频为空，请重新录制"})
        if len(audio) > 20 * 1024 * 1024:
            return JSONResponse(status_code=413, content={"error": "录音过长，请控制在 60 秒以内"})

        model = get_asr_model()
        segments, info = model.transcribe(
            io.BytesIO(audio),
            language=CONFIG["asr_language"],
            beam_size=5,
            vad_filter=True,
            vad_parameters={"min_silence_duration_ms": 500},
            initial_prompt=ASR_INITIAL_PROMPT,
            condition_on_previous_text=False,
        )
        text = "".join(segment.text for segment in segments).strip()
        if not text:
            return JSONResponse(status_code=422, content={"error": "没有识别到语音，请靠近麦克风再说一次"})

        # 不把识别出的症状文本写进日志，避免健康信息落盘。
        logging.info(f"语音识别完成：{len(audio)} 字节，{info.duration:.1f} 秒，文本长度 {len(text)}")
        return {"text": text, "language": info.language, "duration": round(info.duration, 2)}
    except Exception as e:
        logging.exception(f"语音识别失败: {e}")
        return JSONResponse(status_code=500, content={"error": "语音识别服务暂时不可用，请稍后重试"})


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
                        f"【{candidate['prediction']}】\n{info}"
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
        confidence: float = Form(...),
        memory_context: str = Form("")
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
                    # 医生回答统一用较低温度和固定 top_p，减少同一问题措辞漂移。
                    temperature=CONFIG["temperature"],
                    top_p=CONFIG["top_p"],
                    streaming=True
                )

                # 生成回复
                system_message = SystemMessage(content=SKIN_DOCTOR_SYSTEM_PROMPT)
                response = call_llm(
                    chat=chat,
                    system_message=system_message,
                    name_info=name_info,
                    desc_info=desc_info,
                    question=conversation,
                    predicted_class=predicted_class,
                    confidence=confidence,
                    weight=weight,
                    memory_context=memory_context
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
        confidence: float = Form(0.0),
        memory_context: str = Form("")
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
            # 与 /process_text 保持一致，避免流式和非流式回答风格不同。
            temperature=CONFIG["temperature"],
            top_p=CONFIG["top_p"],
            streaming=True
        )
        system_message = SystemMessage(content=SKIN_DOCTOR_SYSTEM_PROMPT)

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
                    weight=weight,
                    memory_context=memory_context
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


@app.post("/generate_health_archive")
def generate_health_archive_endpoint(
        profile: str = Form(""),
        history: str = Form("")
):
    """供 Go 后端生成个人中心的 AI 健康档案。

    失败时 archive 为空字符串，Go 层会返回“生成失败”的提示。
    用普通 def，让 FastAPI 把阻塞的大模型调用放到线程池执行。
    """
    return {"archive": generate_health_archive(profile, history)}


# ==================== AI 长期记忆接口 ====================
# 这些接口只给 Go 后端内部调用，不直接暴露给浏览器。
# 部署到公网时必须在 .env 里设置 MEMORY_API_TOKEN，并让 Go 端带同样的值。


def _check_memory_token(token: str | None) -> None:
    """校验 Go 后端带来的内部令牌。

    MEMORY_API_TOKEN 为空时不做校验，方便本地开发；公网部署时一定不要留空，
    否则任何人都能按 user_id 读取或删除别人的病史记忆。
    """
    expected = (os.getenv("MEMORY_API_TOKEN") or "").strip()
    if expected and (token or "").strip() != expected:
        raise HTTPException(status_code=401, detail="invalid memory token")


class MemorySearchRequest(BaseModel):
    user_id: str | int
    query: str
    limit: int = 5


class MemoryAddRequest(BaseModel):
    user_id: str | int
    question: str
    chat_id: int | None = None
    source: str = "text"
    prediction: str | None = None


@app.post("/memory/search")
def memory_search_endpoint(
        req: MemorySearchRequest,
        x_memory_token: str | None = Header(default=None)
):
    """检索某个用户的长期记忆，供 Go 在生成回答前调用。"""
    _check_memory_token(x_memory_token)
    return {"results": memory_service.search(req.user_id, req.query, req.limit)}


@app.post("/memory/add")
def memory_add_endpoint(
        req: MemoryAddRequest,
        x_memory_token: str | None = Header(default=None)
):
    """从用户原话中抽取事实并写入记忆，供 Go 在回答完成后异步调用。

    只接收用户问题；assistant 回答故意不传，避免把模型诊断存成事实。
    """
    _check_memory_token(x_memory_token)
    metadata = {"source": (req.source or "text").strip()}
    if req.chat_id is not None:
        metadata["chat_id"] = req.chat_id
    if req.prediction:
        metadata["prediction"] = req.prediction.strip()
    return memory_service.add(req.user_id, req.question, metadata)


@app.get("/memory/list")
def memory_list_endpoint(
        user_id: str,
        limit: int = 50,
        x_memory_token: str | None = Header(default=None)
):
    """查看某个用户已经记住的事实，便于本地排查和后续做“清除记忆”功能。"""
    _check_memory_token(x_memory_token)
    return {"results": memory_service.list(user_id, limit)}


@app.delete("/memory/user/{user_id}")
def memory_clear_endpoint(
        user_id: str,
        x_memory_token: str | None = Header(default=None)
):
    """清空某个用户的全部长期记忆。"""
    _check_memory_token(x_memory_token)
    return {"ok": memory_service.clear(user_id)}


if __name__ == "__main__":
    # 运行 FastAPI 应用；端口可通过 PORT 环境变量覆盖。
    uvicorn.run(app, host='0.0.0.0', port=CONFIG["port"])
