# -*- coding: utf-8 -*-
"""AI 问诊的长期记忆封装。

设计目标：
1. 只把用户主动说过的医学事实交给 mem0 提取，不把 AI 回答写进记忆，
   避免模型把自己的猜测当成下一次问诊的事实。
2. 记忆存本地 qdrant + SQLite，不依赖额外服务器，也不改动项目数据库。
3. 任何一步失败都只记日志并返回空结果，绝不影响正常的问诊回答。
"""

from __future__ import annotations

import atexit
import logging
import os
import threading
import time
from pathlib import Path
from typing import Any, Dict, Iterable, List, Optional

BASE_DIR = Path(__file__).resolve().parent

# mem0 在没装 spaCy/fastembed 时会打印“关键词检索增强未启用”的警告。
# 本项目只用向量检索，这些警告没有实际影响，降到 ERROR 避免刷屏。
logging.getLogger("mem0.utils.spacy_models").setLevel(logging.ERROR)
logging.getLogger("mem0.vector_stores.qdrant").setLevel(logging.ERROR)


def _env_bool(name: str, default: bool) -> bool:
    """读取布尔环境变量，兼容 true/1/yes/on 等常见写法。"""
    value = os.getenv(name)
    if value is None or not value.strip():
        return default
    return value.strip().lower() in {"1", "true", "yes", "on"}


def _env_path(name: str, default: Path) -> Path:
    """读取路径环境变量；相对路径统一相对 Python 服务目录解析。"""
    raw = (os.getenv(name) or "").strip()
    if not raw:
        return default
    path = Path(raw)
    return path if path.is_absolute() else BASE_DIR / path


def _env_int(name: str, default: int) -> int:
    try:
        return int((os.getenv(name) or "").strip() or default)
    except ValueError:
        return default


def _env_float(name: str, default: float) -> float:
    try:
        return float((os.getenv(name) or "").strip() or default)
    except ValueError:
        return default


# 记忆只保存用户自述的医学事实，不保存助手诊断、建议或与皮肤无关的内容。
# 必须显式要求简体中文，否则 mem0 的抽取提示词会把记忆写成英文。
CUSTOM_INSTRUCTIONS = (
    "只记录用户本人明确陈述的皮肤相关事实：症状、部位、持续时间、既往病史、"
    "过敏史、正在使用或曾使用的药物、诱发因素。"
    "不要记录用户的提问、疑问或咨询意图（例如“要注意什么”“能不能用”）；"
    "不要记录助手给出的诊断、推测或用药建议；不要记录与皮肤无关的信息。"
    "所有记忆必须用简体中文完整表述，不要翻译成英文。"
)


class MemoryService:
    """mem0 的懒加载封装，供 FastAPI 接口直接调用。"""

    def __init__(self) -> None:
        # 注意：main.py 会先 import 本模块、后执行 load_dotenv()。
        # 因此这里不能立刻读取环境变量，必须等第一次真正使用（或健康检查）时再读，
        # 否则 .env 里的 LLM_API_KEY / MEMORY_* 会被漏掉。
        self._settings_loaded = False
        self._settings_lock = threading.Lock()
        self._memory = None
        self._init_lock = threading.Lock()
        self._add_lock = threading.Lock()
        self._init_failed_at = 0.0
        # 初始化失败后至少等待 60 秒再重试，避免每来一个请求都重新加载模型。
        self._retry_seconds = 60.0

        # 以下字段在 _load_settings() 中填充；先给默认值便于类型阅读。
        self.enabled = True
        self.qdrant_path = BASE_DIR / "memory_data" / "qdrant"
        self.history_db_path = BASE_DIR / "memory_data" / "history.db"
        self.collection_name = "skin_user_memory"
        self.embedding_model = "embedding-3"
        self.embedding_dims = 2048
        self.top_k = 5
        self.threshold = 0.3
        self.llm_model = "glm-4-flash"
        self.llm_base_url = "https://open.bigmodel.cn/api/paas/v4"
        self.llm_api_key = ""

    def _load_settings(self) -> None:
        """第一次使用时读取环境变量；load_dotenv() 执行之后调用才能拿到 .env。"""
        if self._settings_loaded:
            return
        with self._settings_lock:
            if self._settings_loaded:
                return
            self.enabled = _env_bool("MEMORY_ENABLED", True)
            self.qdrant_path = _env_path("MEMORY_QDRANT_PATH", BASE_DIR / "memory_data" / "qdrant")
            self.history_db_path = _env_path(
                "MEMORY_HISTORY_DB", BASE_DIR / "memory_data" / "history.db"
            )
            self.collection_name = (os.getenv("MEMORY_COLLECTION") or "skin_user_memory").strip()
            self.embedding_model = (os.getenv("MEMORY_EMBEDDING_MODEL") or "embedding-3").strip()
            self.embedding_dims = _env_int("MEMORY_EMBEDDING_DIMS", 2048)
            self.top_k = max(1, min(_env_int("MEMORY_TOP_K", 5), 10))
            self.threshold = max(0.0, min(_env_float("MEMORY_THRESHOLD", 0.3), 1.0))
            self.llm_model = (
                os.getenv("MEMORY_LLM_MODEL") or os.getenv("LLM_MODEL") or "glm-4-flash"
            ).strip()
            self.llm_base_url = (
                os.getenv("MEMORY_LLM_BASE_URL")
                or os.getenv("LLM_BASE_URL")
                or "https://open.bigmodel.cn/api/paas/v4"
            ).strip()
            self.llm_api_key = (
                os.getenv("MEMORY_LLM_API_KEY") or os.getenv("LLM_API_KEY") or ""
            ).strip()
            self._settings_loaded = True

            if not self.enabled:
                logging.info("AI 长期记忆已关闭（MEMORY_ENABLED=false）")
            elif not self.llm_api_key:
                # 没有大模型密钥时 mem0 无法做事实抽取和向量化，直接降级为“无记忆”。
                self.enabled = False
                logging.warning("未配置 LLM_API_KEY，AI 长期记忆自动关闭")

    def is_enabled(self) -> bool:
        self._load_settings()
        return bool(self.enabled)

    def _close(self) -> None:
        """进程退出时关闭 mem0；关闭失败不影响已经写完的数据。"""
        memory = self._memory
        if memory is None:
            return
        try:
            memory.close()
        except Exception:  # noqa: BLE001 - 退出阶段不再抛出
            pass

    def warmup(self) -> None:
        """启动时在后台预热，避免第一个问诊请求承担初始化耗时。"""
        if not self.is_enabled():
            return
        self._get_memory()

    def _build_config(self) -> Dict[str, Any]:
        """组装 mem0 配置。

        qdrant 使用本地文件模式，不启动额外服务；history.db 记录记忆变更历史。
        `embedding_dims` 必须和 embedding 模型实际维度一致，切换模型时要清空旧集合。
        """
        self.history_db_path.parent.mkdir(parents=True, exist_ok=True)
        self.qdrant_path.mkdir(parents=True, exist_ok=True)
        # mem0 默认把一些运行文件放到用户目录，这里统一收进项目的 memory_data。
        os.environ.setdefault("MEM0_DIR", str(self.history_db_path.parent))
        # 关闭匿名遥测，避免把问诊内容相关的统计数据发出去。
        os.environ.setdefault("MEM0_TELEMETRY", "false")

        return {
            "vector_store": {
                "provider": "qdrant",
                "config": {
                    "path": str(self.qdrant_path),
                    "collection_name": self.collection_name,
                    "embedding_model_dims": self.embedding_dims,
                    "on_disk": True,
                },
            },
            "llm": {
                "provider": "openai",
                "config": {
                    "model": self.llm_model,
                    "api_key": self.llm_api_key,
                    "openai_base_url": self.llm_base_url,
                    "temperature": 0.1,
                },
            },
            "embedder": {
                "provider": "openai",
                "config": {
                    "model": self.embedding_model,
                    "api_key": self.llm_api_key,
                    "openai_base_url": self.llm_base_url,
                    "embedding_dims": self.embedding_dims,
                },
            },
            "history_db_path": str(self.history_db_path),
            "custom_instructions": CUSTOM_INSTRUCTIONS,
        }

    def _get_memory(self):
        """懒加载 mem0.Memory；依赖缺失或初始化失败时返回 None。"""
        self._load_settings()
        if not self.enabled:
            return None
        if self._memory is not None:
            return self._memory
        # 初始化失败后短暂冷却，避免故障期间反复加载。
        if self._init_failed_at and time.time() - self._init_failed_at < self._retry_seconds:
            return None

        with self._init_lock:
            if self._memory is not None:
                return self._memory
            if self._init_failed_at and time.time() - self._init_failed_at < self._retry_seconds:
                return None
            try:
                # 延迟导入：没安装 mem0 时，图片/文字问诊仍能正常启动。
                from mem0 import Memory

                self._memory = Memory.from_config(self._build_config())
                logging.info(
                    "AI 长期记忆初始化完成：collection=%s, embedding=%s(%s维)",
                    self.collection_name,
                    self.embedding_model,
                    self.embedding_dims,
                )
                # 进程退出时主动关闭 qdrant 本地客户端，避免解释器关停阶段再报错。
                atexit.register(self._close)
            except Exception as exc:  # noqa: BLE001 - 记忆功能不能拖垮主服务
                self._init_failed_at = time.time()
                logging.error("AI 长期记忆初始化失败，本次运行将降级为无记忆: %s", exc)
                self._memory = None
            return self._memory

    @staticmethod
    def _user_id(user_id: Any) -> str:
        """mem0 的用户标识统一转成字符串，避免 int/str 混用导致检索不到。"""
        return str(user_id).strip()

    @staticmethod
    def _clean_results(items: Iterable[Dict[str, Any]], with_score: bool) -> List[Dict[str, Any]]:
        cleaned: List[Dict[str, Any]] = []
        for item in items or []:
            memory = str(item.get("memory") or "").strip()
            if not memory:
                continue
            row: Dict[str, Any] = {"memory": memory}
            if with_score:
                try:
                    row["score"] = round(float(item.get("score") or 0.0), 4)
                except (TypeError, ValueError):
                    row["score"] = 0.0
            row["created_at"] = item.get("created_at") or ""
            cleaned.append(row)
        return cleaned

    def search(self, user_id: Any, query: str, limit: Optional[int] = None) -> List[Dict[str, Any]]:
        """检索某个用户的长期记忆；失败返回空列表。"""
        memory = self._get_memory()
        query = (query or "").strip()
        if memory is None or not query:
            return []
        top_k = max(1, min(int(limit or self.top_k), 10))
        try:
            result = memory.search(
                query,
                filters={"user_id": self._user_id(user_id)},
                top_k=top_k,
                threshold=self.threshold,
            )
            return self._clean_results(result.get("results", []), with_score=True)
        except Exception as exc:  # noqa: BLE001
            logging.error("长期记忆检索失败: %s", exc)
            return []

    def add(self, user_id: Any, question: str, metadata: Optional[Dict[str, Any]] = None) -> Dict[str, Any]:
        """从用户原话中抽取事实并写入记忆。

        注意只传 role=user 的消息：如果把助手回答一起传进去，模型可能把
        “可能是湿疹”这类推测也存成事实，下一次问诊会被误导。
        """
        memory = self._get_memory()
        question = (question or "").strip()
        if memory is None or len(question) < 2:
            return {"results": []}

        # 写入串行化：mem0 需要先查重再决定 ADD/UPDATE，并发写容易产生重复事实。
        # 检索不加锁，仍可并发响应。
        safe_metadata = {
            key: value
            for key, value in (metadata or {}).items()
            if value is not None and isinstance(value, (str, int, float, bool))
        }
        try:
            with self._add_lock:
                result = memory.add(
                    [{"role": "user", "content": question}],
                    user_id=self._user_id(user_id),
                    metadata=safe_metadata or None,
                )
            rows = []
            for item in result.get("results", []) or []:
                rows.append(
                    {
                        "event": item.get("event") or "",
                        "memory": item.get("memory") or "",
                    }
                )
            return {"results": rows}
        except Exception as exc:  # noqa: BLE001
            logging.error("长期记忆写入失败: %s", exc)
            return {"results": [], "error": "记忆写入失败"}

    def list(self, user_id: Any, limit: int = 50) -> List[Dict[str, Any]]:
        """列出某个用户已经记住的事实，便于本地排查。"""
        memory = self._get_memory()
        if memory is None:
            return []
        try:
            result = memory.get_all(
                filters={"user_id": self._user_id(user_id)},
                top_k=max(1, min(int(limit), 200)),
            )
            return self._clean_results(result.get("results", []), with_score=False)
        except Exception as exc:  # noqa: BLE001
            logging.error("长期记忆列表读取失败: %s", exc)
            return []

    def clear(self, user_id: Any) -> bool:
        """清空某个用户的全部长期记忆。"""
        memory = self._get_memory()
        if memory is None:
            return False
        try:
            with self._add_lock:
                memory.delete_all(user_id=self._user_id(user_id))
            return True
        except Exception as exc:  # noqa: BLE001
            logging.error("长期记忆清理失败: %s", exc)
            return False


# 进程内单例：FastAPI 的同步接口会在线程池中调用它。
memory_service = MemoryService()
