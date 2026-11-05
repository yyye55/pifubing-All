# -*- coding: utf-8 -*-
"""在 prepare_dataset.py 生成的清单上训练 15 类皮肤分类模型。

特点：
- 直接复用 test.py 里的 B3Transformer，保证训练结构和线上推理完全一致。
- 支持两种初始化：从现有 B3_Transformer.pth 微调，或从 ImageNet 预训练主干开始。
- 使用清单 CSV，不再依赖 sorted(os.listdir())，避免类别顺序错位。
- 验证集按 macro-F1 选最佳模型，测试集只在最后评估。
- 保存的 best.pth 是纯 state_dict，可以直接替换现有 B3_Transformer.pth。

用法：
    # 微调现有权重（推荐先做）
    python training/train.py --init B3_Transformer.pth --epochs 20

    # 从 ImageNet 主干重新训练
    python training/train.py --scratch --epochs 40
"""

import argparse
import csv
import json
import random
import sys
import time
from collections import Counter, defaultdict
from pathlib import Path

import numpy as np
import torch
import torch.nn as nn
import torch.nn.functional as F
from PIL import Image
from torch.utils.data import DataLoader, Dataset
from torchvision import transforms
from torchvision.models import EfficientNet_B3_Weights, efficientnet_b3

# 让脚本能 import 上一级目录里的 test.py（模型定义）。
sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import test as T  # noqa: E402

IMAGENET_MEAN = [0.485, 0.456, 0.406]
IMAGENET_STD = [0.229, 0.224, 0.225]


class ManifestDataset(Dataset):
    """从 CSV 清单读取 (图片路径, 类别下标)。"""

    def __init__(self, rows, raw_root: Path, transform):
        self.rows = rows
        self.raw_root = raw_root
        self.transform = transform

    def __len__(self):
        return len(self.rows)

    def __getitem__(self, idx):
        row = self.rows[idx]
        path = self.raw_root / row["path"]
        try:
            with Image.open(path) as image:
                image = image.convert("RGB")
                image = self.transform(image)
        except Exception as exc:
            raise RuntimeError(f"读取图片失败: {path}") from exc
        return image, int(row["label"])


class BalancedFocalLoss(nn.Module):
    """按类别频率加权的 Focal Loss，缓解类别不平衡。"""

    def __init__(self, class_counts, gamma=2.0, alpha_ratio=0.8):
        super().__init__()
        counts = torch.tensor(class_counts, dtype=torch.float32)
        weights = 1.0 / (counts + 1e-6)
        alpha = torch.pow(weights / weights.max(), alpha_ratio)
        self.register_buffer("alpha", alpha)
        self.gamma = gamma

    def forward(self, logits, targets):
        ce_loss = F.cross_entropy(logits, targets, reduction="none")
        pt = torch.exp(-ce_loss)
        loss = self.alpha[targets] * (1 - pt) ** self.gamma * ce_loss
        return loss.mean()


def read_manifest(path: Path):
    rows = []
    with open(path, "r", encoding="utf-8-sig") as fh:
        for row in csv.DictReader(fh):
            row["label"] = int(row["label"])
            rows.append(row)
    return rows


def sample_per_class(rows, per_class, seed):
    """每类抽取固定数量，用于小规模冒烟测试。"""
    by_label = defaultdict(list)
    for row in rows:
        by_label[row["label"]].append(row)
    rng = random.Random(seed)
    sampled = []
    for label in sorted(by_label):
        items = sorted(by_label[label], key=lambda r: r["path"])
        rng.shuffle(items)
        sampled.extend(items[:per_class])
    return sampled


def build_transforms(image_size: int):
    train_transform = transforms.Compose([
        transforms.RandomResizedCrop(
            image_size, scale=(0.65, 1.0), ratio=(0.8, 1.25)),
        transforms.RandomHorizontalFlip(),
        transforms.RandomRotation(15),
        transforms.ColorJitter(
            brightness=0.15, contrast=0.15, saturation=0.15, hue=0.02),
        transforms.ToTensor(),
        transforms.Normalize(IMAGENET_MEAN, IMAGENET_STD),
        transforms.RandomErasing(p=0.25, scale=(0.02, 0.08)),
    ])
    # 验证/测试的预处理要和 test.py 的推理保持一致：
    # 短边缩放到 image_size，再从中心裁成正方形。
    eval_transform = transforms.Compose([
        transforms.Resize(image_size),
        transforms.CenterCrop(image_size),
        transforms.ToTensor(),
        transforms.Normalize(IMAGENET_MEAN, IMAGENET_STD),
    ])
    return train_transform, eval_transform


def macro_f1(y_true, y_pred, num_classes):
    """不依赖 sklearn 的 macro-F1。"""
    f1_scores = []
    for cls in range(num_classes):
        tp = sum(1 for t, p in zip(y_true, y_pred) if t == cls and p == cls)
        fp = sum(1 for t, p in zip(y_true, y_pred) if t != cls and p == cls)
        fn = sum(1 for t, p in zip(y_true, y_pred) if t == cls and p != cls)
        precision = tp / (tp + fp) if tp + fp else 0.0
        recall = tp / (tp + fn) if tp + fn else 0.0
        f1 = (2 * precision * recall / (precision + recall)
              if precision + recall else 0.0)
        f1_scores.append(f1)
    return float(np.mean(f1_scores))


def run_epoch(model, loader, criterion, device, num_classes,
              optimizer=None, scaler=None, accum_steps=1, use_amp=False):
    """训练或验证一轮。optimizer 为 None 时表示验证。"""
    training = optimizer is not None
    model.train(training)
    freeze_blocks = getattr(model, "freeze_blocks", 0)
    if training and freeze_blocks > 0:
        # 冻结的浅层保持 eval，避免 BN 统计量被小 batch 带偏。
        model.cnn.features[:freeze_blocks].eval()

    total_loss = 0.0
    total_n = 0
    y_true, y_pred = [], []
    top3_hits = 0

    for step, (images, labels) in enumerate(loader):
        images = images.to(device, non_blocking=True)
        labels = labels.to(device, non_blocking=True)

        with torch.set_grad_enabled(training):
            with torch.amp.autocast("cuda", enabled=use_amp):
                logits = model(images)
                loss = criterion(logits, labels)
            if training:
                loss = loss / accum_steps
                scaler.scale(loss).backward()
                if (step + 1) % accum_steps == 0 or (step + 1) == len(loader):
                    scaler.unscale_(optimizer)
                    torch.nn.utils.clip_grad_norm_(model.parameters(), 5.0)
                    scaler.step(optimizer)
                    scaler.update()
                    optimizer.zero_grad(set_to_none=True)

        batch_n = labels.size(0)
        total_loss += float(loss.item()) * accum_steps * batch_n if training else float(loss.item()) * batch_n
        total_n += batch_n

        preds = logits.detach().argmax(dim=1)
        y_true.extend(labels.detach().cpu().tolist())
        y_pred.extend(preds.cpu().tolist())
        top3 = logits.detach().topk(min(3, num_classes), dim=1).indices
        top3_hits += int((top3 == labels.view(-1, 1)).any(dim=1).sum().item())

    top1 = sum(1 for t, p in zip(y_true, y_pred) if t == p) / total_n if total_n else 0.0
    return {
        "loss": total_loss / total_n if total_n else 0.0,
        "top1": top1,
        "top3": top3_hits / total_n if total_n else 0.0,
        "macro_f1": macro_f1(y_true, y_pred, num_classes),
        "n": total_n,
    }


def load_initial_weights(model, init_path: Path, scratch: bool):
    if scratch:
        # 只加载 ImageNet 主干；分类头仍然是 15 类随机初始化。
        imagenet = efficientnet_b3(weights=EfficientNet_B3_Weights.IMAGENET1K_V1)
        missing, unexpected = model.cnn.load_state_dict(
            imagenet.state_dict(), strict=False)
        print(f"ImageNet 主干加载完成，缺失键 {len(missing)}，多余键 {len(unexpected)}")
        return
    if not init_path.exists():
        raise FileNotFoundError(f"初始化权重不存在: {init_path}")
    state = torch.load(init_path, map_location="cpu", weights_only=False)
    model.load_state_dict(state)
    print(f"已加载初始化权重: {init_path}")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--manifest-dir",
                        default=r"D:\皮肤病相关资料\pifubing-data\manifests_15")
    parser.add_argument("--raw-root",
                        default=r"D:\皮肤病相关资料\pifubing-data\skin_data")
    parser.add_argument(
        "--out-dir",
        default=str(Path(__file__).resolve().parent / "runs" / "b3_finetune"))
    parser.add_argument(
        "--init",
        default=str(Path(__file__).resolve().parents[1] / "B3_Transformer.pth"),
        help="微调时的初始权重；--scratch 时忽略")
    parser.add_argument("--scratch", action="store_true",
                        help="从 ImageNet 主干重新训练，不加载现有权重")
    parser.add_argument("--epochs", type=int, default=20)
    parser.add_argument("--batch-size", type=int, default=8)
    parser.add_argument("--accum-steps", type=int, default=2)
    parser.add_argument("--image-size", type=int, default=300)
    parser.add_argument("--lr", type=float, default=1e-4,
                        help="Transformer/融合层/分类头学习率")
    parser.add_argument("--backbone-lr", type=float, default=1e-5)
    parser.add_argument("--weight-decay", type=float, default=1e-4)
    parser.add_argument("--freeze-blocks", type=int, default=5)
    parser.add_argument("--patience", type=int, default=8)
    parser.add_argument("--num-workers", type=int, default=0,
                        help="Windows 建议 0，避免多进程读图问题")
    parser.add_argument("--seed", type=int, default=42)
    parser.add_argument("--monitor", default="macro_f1",
                        choices=["macro_f1", "top1"])
    parser.add_argument("--limit-train", type=int, default=0,
                        help="冒烟测试：每类只取 N 张训练图，0 表示全部")
    parser.add_argument("--limit-val", type=int, default=0,
                        help="冒烟测试：每类只取 N 张验证图，0 表示全部")
    args = parser.parse_args()

    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8")

    random.seed(args.seed)
    np.random.seed(args.seed)
    torch.manual_seed(args.seed)
    if torch.cuda.is_available():
        torch.cuda.manual_seed_all(args.seed)

    device = torch.device("cuda" if torch.cuda.is_available() else "cpu")
    use_amp = device.type == "cuda"
    print(f"设备: {device}，AMP: {use_amp}")

    manifest_dir = Path(args.manifest_dir)
    raw_root = Path(args.raw_root)
    out_dir = Path(args.out_dir)
    out_dir.mkdir(parents=True, exist_ok=True)

    train_rows = read_manifest(manifest_dir / "train.csv")
    val_rows = read_manifest(manifest_dir / "val.csv")
    if args.limit_train > 0:
        train_rows = sample_per_class(train_rows, args.limit_train, args.seed)
    if args.limit_val > 0:
        val_rows = sample_per_class(val_rows, args.limit_val, args.seed)
    if not train_rows or not val_rows:
        raise RuntimeError("训练/验证清单为空，请先运行 prepare_dataset.py")

    num_classes = len(T.DEFAULT_CLASS_NAMES)
    class_counts = []
    counter = Counter(row["label"] for row in train_rows)
    for idx in range(num_classes):
        class_counts.append(counter.get(idx, 0))
    print(f"训练集 {len(train_rows)} 张，验证集 {len(val_rows)} 张，"
          f"类别数 {num_classes}")
    print("训练集类别分布:",
          {T.DEFAULT_CLASS_NAMES[i]: class_counts[i] for i in range(num_classes)})

    train_transform, eval_transform = build_transforms(args.image_size)
    train_loader = DataLoader(
        ManifestDataset(train_rows, raw_root, train_transform),
        batch_size=args.batch_size, shuffle=True,
        num_workers=args.num_workers, pin_memory=use_amp, drop_last=True,
    )
    val_loader = DataLoader(
        ManifestDataset(val_rows, raw_root, eval_transform),
        batch_size=args.batch_size * 2, shuffle=False,
        num_workers=args.num_workers, pin_memory=use_amp,
    )

    model = T.B3Transformer(num_classes=num_classes)
    load_initial_weights(model, Path(args.init), args.scratch)

    if args.freeze_blocks > 0:
        frozen = model.cnn.features[:args.freeze_blocks]
        for param in frozen.parameters():
            param.requires_grad = False
        # 只记录一个整数，避免把同一个子模块重复注册到 state_dict 里。
        model.freeze_blocks = args.freeze_blocks
        print(f"已冻结 CNN 前 {args.freeze_blocks} 个 block")

    model = model.to(device)

    criterion = BalancedFocalLoss(class_counts).to(device)
    optimizer = torch.optim.AdamW([
        {"params": model.cnn.parameters(), "lr": args.backbone_lr},
        {"params": model.transformer.parameters(), "lr": args.lr},
        {"params": model.fusion.parameters(), "lr": args.lr},
        {"params": model.classifier.parameters(), "lr": args.lr},
    ], weight_decay=args.weight_decay)
    scheduler = torch.optim.lr_scheduler.CosineAnnealingLR(
        optimizer, T_max=args.epochs)
    scaler = torch.amp.GradScaler("cuda", enabled=use_amp)

    best_score = -1.0
    best_epoch = -1
    patience_left = args.patience
    history = []
    start_time = time.time()

    for epoch in range(1, args.epochs + 1):
        epoch_start = time.time()
        train_metrics = run_epoch(
            model, train_loader, criterion, device, num_classes,
            optimizer=optimizer, scaler=scaler,
            accum_steps=args.accum_steps, use_amp=use_amp)
        val_metrics = run_epoch(
            model, val_loader, criterion, device, num_classes,
            optimizer=None, scaler=None, accum_steps=1, use_amp=use_amp)
        scheduler.step()

        score = val_metrics[args.monitor]
        row = {
            "epoch": epoch,
            "train_loss": round(train_metrics["loss"], 4),
            "train_top1": round(train_metrics["top1"], 4),
            "val_loss": round(val_metrics["loss"], 4),
            "val_top1": round(val_metrics["top1"], 4),
            "val_top3": round(val_metrics["top3"], 4),
            "val_macro_f1": round(val_metrics["macro_f1"], 4),
            "lr": optimizer.param_groups[0]["lr"],
            "seconds": round(time.time() - epoch_start, 1),
        }
        history.append(row)
        print(json.dumps(row, ensure_ascii=False))

        torch.save(model.state_dict(), out_dir / "last.pth")
        if score > best_score:
            best_score = score
            best_epoch = epoch
            patience_left = args.patience
            torch.save(model.state_dict(), out_dir / "best.pth")
            with open(out_dir / "best_meta.json", "w", encoding="utf-8") as fh:
                json.dump({
                    "epoch": epoch,
                    "monitor": args.monitor,
                    "score": round(score, 4),
                    "val": row,
                    "classes": T.DEFAULT_CLASS_NAMES,
                    "args": vars(args),
                }, fh, ensure_ascii=False, indent=2)
            print(f"  -> 保存最佳模型 best.pth（epoch {epoch}, "
                  f"{args.monitor}={score:.4f}）")
        else:
            patience_left -= 1
            if patience_left <= 0:
                print(f"早停：验证指标连续 {args.patience} 轮没有提升")
                break

    with open(out_dir / "history.csv", "w", newline="", encoding="utf-8-sig") as fh:
        writer = csv.DictWriter(fh, fieldnames=list(history[0].keys()))
        writer.writeheader()
        writer.writerows(history)

    with open(out_dir / "classes.json", "w", encoding="utf-8") as fh:
        json.dump([
            {"index": i, "zh": name} for i, name in enumerate(T.DEFAULT_CLASS_NAMES)
        ], fh, ensure_ascii=False, indent=2)

    print(f"\n训练完成，用时 {(time.time() - start_time) / 60:.1f} 分钟")
    print(f"最佳 epoch: {best_epoch}，{args.monitor}: {best_score:.4f}")
    print(f"最佳权重: {out_dir / 'best.pth'}")
    print("下一步：用 training/evaluate.py 在 test.csv 上评估 best.pth，"
          "确认优于现有模型后再替换 B3_Transformer.pth。")


if __name__ == "__main__":
    main()
