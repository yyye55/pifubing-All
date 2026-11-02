import torch
import torch.nn as nn
import torch.nn.functional as F
from torchvision import models
from torch.nn import TransformerEncoder, TransformerEncoderLayer
import os
import cv2
import numpy as np
from pathlib import Path
from typing import List, Tuple, Optional

# ==================== 默认配置 ====================
DEFAULT_CLASS_NAMES = [
    '痤疮和玫瑰痤疮', '光化性角化病、基底细胞癌及其他皮肤癌', '特应性皮炎',
    '良性痣', '湿疹', '发疹和药物疹', '疱疹、人乳头瘤病毒及其他性传播疾病',
    '光照性疾病和色素沉着障碍', '恶性痣', '甲真菌病及其他指甲疾病',
    '银屑病、扁平苔藓及相关疾病', '脂溢性角化病及其他良性肿瘤',
    '全身性疾病', '体癣、念珠菌病及其他真菌感染', '疣、传染性软疣及其他病毒感染'
]


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

    def _preprocess(self, image_path: str) -> Optional[torch.Tensor]:
        """图像预处理"""
        try:
            # 处理中文路径
            valid_path = Path(image_path).resolve().as_posix()

            # 读取图像
            img = cv2.imdecode(np.fromfile(valid_path, dtype=np.uint8), cv2.IMREAD_COLOR)
            if img is None:
                raise ValueError("图像解码失败")

            # 预处理流程
            img = cv2.cvtColor(img, cv2.COLOR_BGR2RGB)
            h, w = img.shape[:2]
            short_edge = min(h, w)
            img = img[(h - short_edge) // 2:(h + short_edge) // 2, (w - short_edge) // 2:(w + short_edge) // 2]
            img = cv2.resize(img, (self.image_size, self.image_size), interpolation=cv2.INTER_LINEAR)

            # 转换为Tensor
            tensor = torch.from_numpy(img).float().permute(2, 0, 1) / 255.0
            tensor = (tensor - torch.tensor([0.485, 0.456, 0.406]).view(3, 1, 1)) / \
                     torch.tensor([0.229, 0.224, 0.225]).view(3, 1, 1)
            return tensor.unsqueeze(0)

        except Exception as e:
            print(f"预处理错误 [{image_path}]: {str(e)}")
            return None

    def _predict_single(self, image_path: str) -> Optional[Tuple[int, float]]:
        """单张图像预测"""
        try:
            tensor = self._preprocess(image_path)
            if tensor is None:
                return None

            with torch.inference_mode():
                output = self.model(tensor)
                prob = F.softmax(output[0], dim=0)
                conf, class_idx = torch.max(prob, 0)

            return (class_idx.item(), conf.item())
        except Exception as e:
            print(f"预测错误 [{image_path}]: {str(e)}")
            return None

    def predict(self, image_paths: List[str]) -> dict:
        results = []
        valid_results = []

        for path in image_paths:
            pred = self._predict_single(path)
            if pred is not None:
                class_idx, conf = pred
                results.append({
                    "path": path,
                    "class": self.class_names[class_idx],
                    "confidence": conf,
                    "status": "success"
                })
                valid_results.append((class_idx, conf))
            else:
                results.append({
                    "path": path,
                    "class": None,
                    "confidence": None,
                    "status": "failed"
                })

        # 计算最终结果
        final_class, final_conf = self._vote_results(valid_results)

        return {
            "detailed_results": results,
            "final_class": final_class,
            "final_confidence": final_conf
        }

    def _vote_results(self, valid_results: List[Tuple[int, float]]) -> Tuple[Optional[str], Optional[float]]:
        """投票机制"""
        if not valid_results:
            return None, None

        # 统计投票
        vote_dict = {}
        for class_idx, conf in valid_results:
            if class_idx not in vote_dict:
                vote_dict[class_idx] = {"count": 0, "total_conf": 0.0}
            vote_dict[class_idx]["count"] += 1
            vote_dict[class_idx]["total_conf"] += conf

        # 找出最高票数
        max_vote = max(vote_dict.values(), key=lambda x: x["count"])
        candidates = [k for k, v in vote_dict.items() if v["count"] == max_vote["count"]]

        # 如果有平票，选择置信度最高的
        if len(candidates) > 1:
            final_class = max(candidates, key=lambda x: vote_dict[x]["total_conf"])
        else:
            final_class = candidates[0]

        avg_conf = vote_dict[final_class]["total_conf"] / vote_dict[final_class]["count"]
        return self.class_names[final_class], avg_conf


# ==================== 对外接口 ====================
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

    返回: 包含详细结果和最终投票结果的字典
    """
    try:
        classifier = SkinDiseaseClassifier(
            checkpoint_path=checkpoint_path,
            image_size=image_size,
            cpu_threads=cpu_threads
        )
        return classifier.predict(image_paths)
    except Exception as e:
        return {
            "error": str(e),
            "detailed_results": [],
            "final_class": None,
            "final_confidence": None
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