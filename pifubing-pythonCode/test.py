import torch
import torch.nn as nn
import torch.nn.functional as F
from torchvision import models
from torch.nn import TransformerEncoder, TransformerEncoderLayer
import os
import cv2
import numpy as np
import threading
from pathlib import Path
from typing import List, Tuple, Optional

# ==================== 默认配置 ====================
# 返回给上层的候选病种数量；top-1 仍作为最终分类结果，top-2/3 供大模型参考。
TOP_K = 3

# 图片质量门槛。阈值刻意放宽：目标是拦住“完全没法看”的图片，
# 而不是挑刺，避免正常手机照片被误判后无法问诊。
# 所有阈值都可以通过环境变量覆盖，方便按真实数据再调。
QUALITY_MIN_EDGE = int(os.getenv("QUALITY_MIN_EDGE", "120"))
QUALITY_MIN_SHARPNESS = float(os.getenv("QUALITY_MIN_SHARPNESS", "20"))
QUALITY_MIN_BRIGHTNESS = float(os.getenv("QUALITY_MIN_BRIGHTNESS", "35"))
QUALITY_MAX_BRIGHTNESS = float(os.getenv("QUALITY_MAX_BRIGHTNESS", "235"))
QUALITY_MIN_SKIN_RATIO = float(os.getenv("QUALITY_MIN_SKIN_RATIO", "0.08"))

# TTA（测试时增强）默认关闭。实测该模型对翻转/放大非常敏感，
# 在没有验证集确认收益前开启反而可能改变原有结果；配好标注数据后可设 ENABLE_TTA=1 再评估。
ENABLE_TTA = os.getenv("ENABLE_TTA", "0").lower() in ("1", "true", "yes")

DEFAULT_CLASS_NAMES = [
    '痤疮和玫瑰痤疮', '光化性角化病、基底细胞癌及其他皮肤癌', '特应性皮炎',
    '良性痣', '湿疹', '发疹和药物疹', '疱疹、人乳头瘤病毒及其他性传播疾病',
    '光照性疾病和色素沉着障碍', '恶性痣', '甲真菌病及其他指甲疾病',
    '银屑病、扁平苔藓及相关疾病', '脂溢性角化病及其他良性肿瘤',
    '全身性疾病', '体癣、念珠菌病及其他真菌感染', '疣、传染性软疣及其他病毒感染'
]


# ==================== 图片读取与质量检测 ====================
def read_image(image_path: str):
    """读取图片，兼容中文路径；读取失败返回 None。"""
    try:
        valid_path = Path(image_path).resolve().as_posix()
        return cv2.imdecode(np.fromfile(valid_path, dtype=np.uint8), cv2.IMREAD_COLOR)
    except Exception as e:
        print(f"图片读取错误 [{image_path}]: {str(e)}")
        return None


def check_image_quality(image_path: str) -> dict:
    """检查图片是否适合送入分类模型。

    检查项：分辨率、清晰度、亮度、肤色像素占比。
    返回 {"ok": bool, "reasons": [...], "metrics": {...}}，
    前端/Go 端根据 ok=false 提示用户重拍，而不是硬给一个低可信度结果。
    """
    img = read_image(image_path)
    if img is None:
        return {"ok": False, "reasons": ["图片无法读取或格式不支持"], "metrics": {}}

    h, w = img.shape[:2]
    gray = cv2.cvtColor(img, cv2.COLOR_BGR2GRAY)

    # 清晰度统一在最长边 512 的尺度上计算，避免大图因为像素多而虚高。
    if max(h, w) > 512:
        scale = 512.0 / max(h, w)
        gray = cv2.resize(gray, (max(1, int(w * scale)), max(1, int(h * scale))),
                          interpolation=cv2.INTER_AREA)
    sharpness = float(cv2.Laplacian(gray, cv2.CV_64F).var())
    brightness = float(gray.mean())

    # YCrCb 肤色范围是常用的经验区间，只用来判断“画面里有没有皮肤”，
    # 不参与病种判断，阈值放得很宽。
    ycc = cv2.cvtColor(img, cv2.COLOR_BGR2YCrCb)
    _, cr, cb = cv2.split(ycc)
    skin_mask = (cr > 133) & (cr < 173) & (cb > 77) & (cb < 127)
    skin_ratio = float(skin_mask.mean())

    reasons = []
    if min(h, w) < QUALITY_MIN_EDGE:
        reasons.append(f"图片分辨率过低（{w}×{h}），请上传更清晰的照片")
    if sharpness < QUALITY_MIN_SHARPNESS:
        reasons.append("图片可能模糊，请重新对焦拍摄")
    if brightness < QUALITY_MIN_BRIGHTNESS:
        reasons.append("图片过暗，请在自然光下拍摄")
    elif brightness > QUALITY_MAX_BRIGHTNESS:
        reasons.append("图片过亮或反光严重，请避免闪光灯直射")
    if skin_ratio < QUALITY_MIN_SKIN_RATIO:
        reasons.append("画面中没有明显的皮肤区域，请对准患处拍摄")

    return {
        "ok": len(reasons) == 0,
        "reasons": reasons,
        "metrics": {
            "width": w,
            "height": h,
            "sharpness": round(sharpness, 2),
            "brightness": round(brightness, 2),
            "skin_ratio": round(skin_ratio, 4),
        },
    }


# ==================== 模型定义 ====================
class B3Transformer(nn.Module):
    """混合EfficientNetB3+Transformer模型"""

    def __init__(self, num_classes: int = 15):
        super().__init__()

        # CNN主干网络
        self.cnn = models.efficientnet_b3(weights=None)
        in_features = self.cnn.classifier[1].in_features
        self.cnn.classifier[1] = nn.Linear(in_features, num_classes)

        # Transformer分支
        self.patch_embed = nn.Sequential(
            nn.Conv2d(3, 256, kernel_size=16, stride=16),
            nn.BatchNorm2d(256),
            nn.GELU()
        )

        encoder_layer = TransformerEncoderLayer(
            d_model=256,
            nhead=8,
            dim_feedforward=1024,
            dropout=0.1,
            activation='gelu',
            batch_first=True
        )
        self.transformer = TransformerEncoder(encoder_layer, num_layers=3)

        # 特征融合模块
        self.fusion = nn.Sequential(
            nn.Conv2d(1536 + 256, 1024, 1),
            nn.BatchNorm2d(1024),
            nn.SiLU(),
            nn.Dropout2d(0.3)
        )

        # 分类头
        self.classifier = nn.Sequential(
            nn.AdaptiveAvgPool2d(1),
            nn.Flatten(),
            nn.Linear(1024, 512),
            nn.BatchNorm1d(512),
            nn.SiLU(),
            nn.Dropout(0.5),
            nn.Linear(512, num_classes)
        )

    def forward(self, x: torch.Tensor) -> torch.Tensor:
        # 特征提取与融合
        cnn_feat = self.cnn.features(x)

        patches = self.patch_embed(x)
        B, C, H, W = patches.shape
        trans_feat = self.transformer(patches.view(B, C, -1).permute(0, 2, 1))
        trans_feat = trans_feat.mean(dim=1)[:, :, None, None]
        trans_feat = F.interpolate(trans_feat, size=cnn_feat.shape[2:])

        fused = torch.cat([cnn_feat, trans_feat], dim=1)
        return self.classifier(self.fusion(fused))


# ==================== 核心功能 ====================
class SkinDiseaseClassifier:
    """皮肤病分类器"""

    def __init__(self, checkpoint_path: str, image_size: int = 300, cpu_threads: int = 2):
        # 环境配置
        os.environ["OMP_NUM_THREADS"] = str(cpu_threads)
        os.environ["MKL_NUM_THREADS"] = str(cpu_threads)
        torch.set_num_threads(cpu_threads)

        # 加载模型
        self.model = self._load_model(checkpoint_path)
        self.image_size = image_size
        self.class_names = DEFAULT_CLASS_NAMES

    def _load_model(self, checkpoint_path: str) -> nn.Module:
        """加载预训练模型"""
        try:
            model = B3Transformer(num_classes=len(DEFAULT_CLASS_NAMES))
            state_dict = torch.load(checkpoint_path, map_location='cpu', weights_only=False)
            model.load_state_dict(state_dict)
            model.eval()
            return model
        except Exception as e:
            raise RuntimeError(f"模型加载失败: {str(e)}")

    def _preprocess(self, image_path: str, crop_scale: float = 1.0,
                    flip: bool = False) -> Optional[torch.Tensor]:
        """图像预处理：中心裁成正方形后缩放（与训练时的 RandomResizedCrop 一致）。

        crop_scale < 1.0 表示从中心再轻微放大裁切，flip=True 表示水平翻转，
        这两种视图只用于 TTA，不会单独决定结果。
        """
        try:
            img = read_image(image_path)
            if img is None:
                raise ValueError("图像解码失败")

            img = cv2.cvtColor(img, cv2.COLOR_BGR2RGB)
            h, w = img.shape[:2]
            short_edge = min(h, w)
            side = max(1, int(round(short_edge * crop_scale)))
            x0 = max(0, (w - side) // 2)
            y0 = max(0, (h - side) // 2)
            img = img[y0:y0 + side, x0:x0 + side]
            if flip:
                img = cv2.flip(img, 1)
            img = cv2.resize(img, (self.image_size, self.image_size),
                             interpolation=cv2.INTER_LINEAR)

            # 转换为Tensor
            tensor = torch.from_numpy(img).float().permute(2, 0, 1) / 255.0
            tensor = (tensor - torch.tensor([0.485, 0.456, 0.406]).view(3, 1, 1)) / \
                     torch.tensor([0.229, 0.224, 0.225]).view(3, 1, 1)
            return tensor.unsqueeze(0)

        except Exception as e:
            print(f"预处理错误 [{image_path}]: {str(e)}")
            return None

    def _predict_single(self, image_path: str) -> Optional[Tuple[int, float, np.ndarray]]:
        """单张图像预测，返回 (top1 类别下标, top1 概率, 全部类别的概率向量)。

        ENABLE_TTA=1 时会对“原裁切 / 翻转 / 轻微放大 / 放大后翻转”4 个视图
        的概率取平均；默认只跑原始中心裁切，保证线上结果与旧版一致。
        """
        try:
            views = [(1.0, False)]
            if ENABLE_TTA:
                views = [(1.0, False), (1.0, True), (0.85, False), (0.85, True)]

            view_probs = []
            for crop_scale, flip in views:
                tensor = self._preprocess(image_path, crop_scale=crop_scale, flip=flip)
                if tensor is None:
                    continue
                with torch.inference_mode():
                    output = self.model(tensor)
                    view_probs.append(F.softmax(output[0], dim=0).cpu().numpy())

            if not view_probs:
                return None

            prob = np.mean(np.stack(view_probs), axis=0)
            class_idx = int(np.argmax(prob))
            return (class_idx, float(prob[class_idx]), prob)
        except Exception as e:
            print(f"预测错误 [{image_path}]: {str(e)}")
            return None

    def predict(self, image_paths: List[str]) -> dict:
        results = []
        valid_results = []

        for path in image_paths:
            pred = self._predict_single(path)
            if pred is not None:
                class_idx, conf, probs = pred
                results.append({
                    "path": path,
                    "class": self.class_names[class_idx],
                    "confidence": conf,
                    "status": "success"
                })
                valid_results.append((class_idx, conf, probs))
            else:
                results.append({
                    "path": path,
                    "class": None,
                    "confidence": None,
                    "status": "failed"
                })

        # 多图时对概率取平均（软投票），比原来的硬投票更稳定；
        # 单图时平均后就是该图自身的概率。
        final_class, final_conf, candidates = self._aggregate_results(valid_results)

        return {
            "detailed_results": results,
            "final_class": final_class,
            "final_confidence": final_conf,
            "candidates": candidates
        }

    def _aggregate_results(self, valid_results: List[Tuple[int, float, np.ndarray]]):
        """把多张图的概率取平均，并输出 top-k 候选。"""
        if not valid_results:
            return None, None, []

        mean_prob = np.mean(np.stack([probs for _, _, probs in valid_results]), axis=0)
        final_idx = int(np.argmax(mean_prob))
        final_conf = float(mean_prob[final_idx])

        top_indices = np.argsort(mean_prob)[::-1][:TOP_K]
        candidates = [
            {
                "prediction": self.class_names[int(idx)],
                "confidence": float(mean_prob[int(idx)])
            }
            for idx in top_indices
        ]
        return self.class_names[final_idx], final_conf, candidates


# ==================== 对外接口 ====================
# 模型加载一次大约要读 63MB 权重，每次请求都重新构造会明显拖慢问诊。
# 按“权重路径 + 输入尺寸 + 线程数”缓存实例，进程内复用。
_CLASSIFIER_CACHE = {}
_CLASSIFIER_LOCK = threading.Lock()


def _get_classifier(checkpoint_path: str, image_size: int, cpu_threads: int) -> SkinDiseaseClassifier:
    key = (str(Path(checkpoint_path).resolve()), image_size, cpu_threads)
    with _CLASSIFIER_LOCK:
        if key not in _CLASSIFIER_CACHE:
            _CLASSIFIER_CACHE[key] = SkinDiseaseClassifier(
                checkpoint_path=checkpoint_path,
                image_size=image_size,
                cpu_threads=cpu_threads
            )
        return _CLASSIFIER_CACHE[key]


def predict_images(
        image_paths: List[str],
        checkpoint_path: str = "B3_Transformer.pth",
        image_size: int = 300,
        cpu_threads: int = 2
) -> dict:
    """皮肤病分类预测接口
    参数:
    image_paths: 需要预测的图像路径列表
    checkpoint_path: 模型权重文件路径 (默认当前目录)
    image_size: 输入图像尺寸 (默认300)
    cpu_threads: CPU线程数 (默认2)

    返回: 包含详细结果、最终分类和 top-k 候选的字典
    """
    try:
        classifier = _get_classifier(checkpoint_path, image_size, cpu_threads)
        return classifier.predict(image_paths)
    except Exception as e:
        return {
            "error": str(e),
            "detailed_results": [],
            "final_class": None,
            "final_confidence": None,
            "candidates": []
        }


# ==================== 测试用例 ====================
if __name__ == "__main__":
    # 示例用法
    test_images = [
        r"D:\学习资料\实验室\皮肤病\skin_disease_2.0\data\test\Eczema Photos\Dyshidrosis-2.jpg",
        r"D:\学习资料\实验室\皮肤病\skin_disease_2.0\data\train\Eczema Photos\03AnalExcoriation0512041.jpg",
        r"C:\Users\hasee\Pictures\Screenshots\屏幕截图 2024-10-20 204109.png"
    ]

    result = predict_images(
        image_paths=test_images,
        checkpoint_path="B3_Transformer.pth",
        image_size=300,
        cpu_threads=2
    )

    print("\n预测结果详情:")
    for i, res in enumerate(result["detailed_results"], start=1):
        print(f"第{i}张图片 => {res['class']} ({res['confidence']:.2%})")

    print("\n最终综合结果:")
    print(f"预测类别: {result['final_class']}")
    print(f"预测置信度: {result['final_confidence']:.2%}")