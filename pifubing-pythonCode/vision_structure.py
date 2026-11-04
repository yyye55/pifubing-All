# -*- coding: utf-8 -*-
"""用视觉大模型提取图片中的形态学结构。

只描述“看到了什么”（数量、分布、边界、颜色、表面、是否隆起/鳞屑等），
不输出诊断或病名，避免模型越过分类器和医生直接下结论。

可通过环境变量控制：
  VISION_ENABLED=0        关闭视觉结构化（默认开启）
  VISION_MODEL           默认 glm-4v-flash
  VISION_TIMEOUT         默认 25 秒
  VISION_MAX_EDGE        默认 1024，发送前压缩长边，减少上传耗时
  VISION_API_KEY / VISION_BASE_URL  默认复用 LLM_API_KEY / LLM_BASE_URL
"""
import base64
import json
import logging
import os
import re

import cv2
import numpy as np
import requests

STRUCTURE_FIELDS = (
    "lesion_present",
    "location",
    "pattern",
    "lesion_count",
    "lesion_size",
    "border",
    "color",
    "surface",
    "distribution",
    "central_clearing",
    "elevation",
    "scale",
    "summary",
)

PROMPT = (
    "你是皮肤科图像观察助手。只描述图片中可见的形态学结构，"
    "不要给出诊断、病名、治疗建议或预后判断。\n"
    "请严格输出一个 JSON 对象，不要输出解释、编号或代码块。字段如下：\n"
    "lesion_present：有/无/不确定（图片中是否有明确的皮损）\n"
    "location：皮损部位，如面部/颈部/手背/前臂/躯干/下肢；看不清填“不确定”\n"
    "pattern：局限/多发/环形/弥漫/散在\n"
    "lesion_count：大致数量或范围\n"
    "lesion_size：小/中/大（相对画面）\n"
    "border：边界清晰度与规则性，如“清晰规则”“模糊不规则”\n"
    "color：主要颜色，如红/白/褐/黑/肤色\n"
    "surface：表面特征，如光滑/粗糙/鳞屑/隆起/水疱/结痂\n"
    "distribution：单发/散在/融合/对称/沿某部位分布\n"
    "central_clearing：有/无/不确定（病灶中央是否颜色变浅或消退）\n"
    "elevation：有/无/不确定（是否隆起、风团样或水肿）\n"
    "scale：有/无/不确定（是否有鳞屑/脱屑）\n"
    "summary：一句中文，只描述结构，不下诊断。\n"
    "如果图片中没有明确皮损，lesion_present 填“无”，其余字段全部填“不确定”；"
    "如果 lesion_present 填“有”，其他字段不要写“未见明显皮损”。"
)


def _vision_config():
    enabled = os.getenv("VISION_ENABLED", "1").lower() not in ("0", "false", "no")
    api_key = os.getenv("VISION_API_KEY") or os.getenv("LLM_API_KEY", "")
    base_url = (os.getenv("VISION_BASE_URL") or os.getenv(
        "LLM_BASE_URL", "https://open.bigmodel.cn/api/paas/v4"
    )).rstrip("/")
    return {
        "enabled": enabled,
        "api_key": api_key,
        "base_url": base_url,
        "model": os.getenv("VISION_MODEL", "glm-4v-flash"),
        "timeout": float(os.getenv("VISION_TIMEOUT", "25")),
        "max_edge": int(os.getenv("VISION_MAX_EDGE", "1024")),
    }


def _encode_image(image_path, max_edge):
    """读取图片并压缩成 JPEG base64；返回 (data_uri, 原图宽高) 或 (None, None)。"""
    try:
        data = np.fromfile(image_path, dtype=np.uint8)
        img = cv2.imdecode(data, cv2.IMREAD_COLOR)
        if img is None:
            return None, None
        h, w = img.shape[:2]
        # 长边控制在 768~max_edge：小图放大后视觉模型更容易看清皮损细节，
        # 大图压缩后减少上传耗时。
        target_edge = min(max_edge, max(768, max(h, w)))
        scale = target_edge / max(h, w)
        if abs(scale - 1.0) > 0.01:
            interpolation = cv2.INTER_CUBIC if scale > 1.0 else cv2.INTER_AREA
            img = cv2.resize(img, (max(1, int(w * scale)), max(1, int(h * scale))),
                             interpolation=interpolation)
        ok, buf = cv2.imencode(".jpg", img, [int(cv2.IMWRITE_JPEG_QUALITY), 85])
        if not ok:
            return None, None
        return "data:image/jpeg;base64," + base64.b64encode(buf.tobytes()).decode("ascii"), [w, h]
    except Exception as exc:
        logging.warning(f"视觉结构化图片编码失败: {exc}")
        return None, None


def _parse_json(content):
    """兼容模型偶尔返回 ```json 代码块或前后带解释文字的情况。"""
    text = (content or "").strip()
    match = re.search(r"\{[\s\S]*\}", text)
    if match:
        text = match.group(0)
    return json.loads(text)


def _clean(value, limit=120):
    if value is None:
        return ""
    text = str(value).replace("\n", " ").strip()
    return text[:limit]


_EMPTY_VALUES = ("不确定", "不适用", "无法判断", "未见明显皮损")


def _val(result, key):
    """读取字段值，过滤模型常用的“无信息”占位词。"""
    value = result.get(key, "")
    return "" if value in _EMPTY_VALUES else value


def _compose_summary(result):
    """用结构化字段拼出顺序稳定、无重复的中文形态描述。"""
    pieces = []
    location = _val(result, "location")
    pattern = _val(result, "pattern")
    distribution = _val(result, "distribution")

    head = ""
    if pattern:
        head = "局限性皮损" if pattern == "局限" else f"{pattern}皮损"
        if distribution in ("单发", "融合", "对称") and distribution not in head:
            head = f"{distribution}{head}"
    elif distribution:
        head = f"{distribution}皮损"

    if location and head:
        pieces.append(location + head)
    elif location:
        pieces.append(location)
    elif head:
        pieces.append(head)

    count = _val(result, "lesion_count")
    # “多发皮损，多个”“弥漫皮损，广泛”属于重复信息，去掉数量描述。
    if pattern in ("多发", "弥漫", "散在") and count in ("多个", "数个", "广泛", "大面积"):
        count = ""
    if count:
        pieces.append(count)

    size = _val(result, "lesion_size")
    size_map = {"小": "体积较小", "中": "中等大小", "中等": "中等大小", "大": "体积较大"}
    if size:
        pieces.append(size_map.get(size, size))

    border = _val(result, "border")
    if border:
        pieces.append(border if border.startswith("边界") else f"边界{border}")

    color = _val(result, "color")
    if color:
        pieces.append(color if color.startswith("呈") else f"呈{color}")

    # 模型有时用英文逗号分隔多个表面特征，统一成顿号。
    surface = re.sub(r"\s*[,，]\s*", "、", _val(result, "surface"))
    if surface:
        pieces.append(surface if surface.startswith("表面") else f"表面{surface}")

    if result.get("central_clearing") == "有":
        pieces.append("中央消退")
    if result.get("elevation") == "有":
        pieces.append("局部隆起或风团样")
    if result.get("scale") == "有" and "鳞屑" not in surface and "脱屑" not in surface:
        pieces.append("附有鳞屑")

    return "，".join(pieces)


def describe_image_structure(image_path):
    """调用视觉模型提取结构。

    返回字典：
      {"ok": True, "model": "...", "pattern": "...", ..., "summary": "..."}
    失败时返回 {"ok": False, "error": "...", "summary": ""}，
    调用方应降级为“无结构信息”，不能因此阻塞问诊。
    """
    cfg = _vision_config()
    if not cfg["enabled"]:
        return {"ok": False, "disabled": True, "summary": ""}
    if not cfg["api_key"]:
        return {"ok": False, "error": "未配置视觉模型 API Key", "summary": ""}

    data_uri, image_size = _encode_image(image_path, cfg["max_edge"])
    if not data_uri:
        return {"ok": False, "error": "图片编码失败", "summary": ""}

    payload = {
        "model": cfg["model"],
        "messages": [{
            "role": "user",
            "content": [
                {"type": "text", "text": PROMPT},
                {"type": "image_url", "image_url": {"url": data_uri}},
            ],
        }],
        "temperature": 0.1,
    }

    try:
        response = requests.post(
            f"{cfg['base_url']}/chat/completions",
            headers={
                "Authorization": f"Bearer {cfg['api_key']}",
                "Content-Type": "application/json",
            },
            json=payload,
            timeout=cfg["timeout"],
        )
        if response.status_code != 200:
            logging.warning(f"视觉结构化接口返回 {response.status_code}: {response.text[:200]}")
            return {"ok": False, "error": f"HTTP {response.status_code}", "summary": ""}

        content = response.json()["choices"][0]["message"].get("content", "")
        parsed = _parse_json(content)
    except Exception as exc:
        logging.warning(f"视觉结构化失败: {exc}")
        return {"ok": False, "error": str(exc), "summary": ""}

    # 模型偶尔会返回 JSON 数组或纯文本，不能让结构解析异常拖垮整个问诊。
    if not isinstance(parsed, dict):
        logging.warning(f"视觉结构化返回非 JSON 对象: {str(parsed)[:120]}")
        return {"ok": False, "error": "视觉模型返回格式异常", "summary": ""}

    result = {"ok": True, "model": cfg["model"], "image_size": image_size}
    for field in STRUCTURE_FIELDS:
        result[field] = _clean(parsed.get(field))

    # 模型偶尔会在 pattern 写“多发”、其他字段却写“未见明显皮损”，
    # 或把字段原样复述进 summary。这里统一用字段拼接，保证描述一致、不重复。
    if result["lesion_present"] == "无" or result["pattern"].startswith("未见"):
        result["summary"] = "未识别到明确的皮损结构（可能受清晰度、光线或病灶与肤色接近影响）"
    else:
        composed = _compose_summary(result)
        # 字段都不确定时，退回模型自己的摘要（若有）。
        result["summary"] = composed or result["summary"] or "未获取到可用的图像结构描述"
    logging.info(f"视觉结构: {result['summary']}")
    return result
