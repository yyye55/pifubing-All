# -*- coding: utf-8 -*-
"""在指定清单上评估皮肤分类模型。

用法：
    python training/evaluate.py --manifest D:/皮肤病相关资料/pifubing-data/manifests_15/test.csv
    python training/evaluate.py --manifest ... --per-class 20 --out baseline.json

默认每类抽样 20 张，避免 CPU 上跑全量太慢；--per-class 0 表示跑全部。
"""

import argparse
import csv
import json
import random
import sys
from collections import Counter, defaultdict
from pathlib import Path

import numpy as np
import torch

# 让脚本能 import 上一级目录里的 test.py（模型定义和推理封装）。
sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import test as T  # noqa: E402


def load_manifest(path: Path, per_class: int, seed: int):
    rows = []
    with open(path, "r", encoding="utf-8-sig") as fh:
        for row in csv.DictReader(fh):
            row["label"] = int(row["label"])
            rows.append(row)

    if per_class <= 0:
        return rows

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


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--checkpoint", default="B3_Transformer.pth")
    parser.add_argument("--manifest", required=True)
    parser.add_argument("--raw-root", default=r"D:\皮肤病相关资料\pifubing-data\skin_data")
    parser.add_argument("--per-class", type=int, default=20,
                        help="每类抽样数量，0 表示全部")
    parser.add_argument("--seed", type=int, default=42)
    parser.add_argument("--image-size", type=int, default=300)
    parser.add_argument("--cpu-threads", type=int, default=4)
    parser.add_argument("--device", default="auto",
                        choices=["auto", "cpu", "cuda"])
    parser.add_argument("--tta", action="store_true",
                        help="开启 4 视图 TTA，和 test.py 的 ENABLE_TTA 一致")
    parser.add_argument("--out", default="")
    args = parser.parse_args()

    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8")

    checkpoint = Path(args.checkpoint)
    if not checkpoint.is_absolute():
        checkpoint = Path(__file__).resolve().parents[1] / checkpoint
    raw_root = Path(args.raw_root)
    rows = load_manifest(Path(args.manifest), args.per_class, args.seed)

    print(f"模型权重: {checkpoint}")
    print(f"测试清单: {args.manifest}")
    print(f"评估图片: {len(rows)} 张")

    classifier = T.SkinDiseaseClassifier(
        str(checkpoint), image_size=args.image_size, cpu_threads=args.cpu_threads)
    class_names = classifier.class_names

    if args.device == "auto":
        device = torch.device("cuda" if torch.cuda.is_available() else "cpu")
    else:
        device = torch.device(args.device)
    classifier.model.to(device)
    print(f"推理设备: {device}，TTA: {args.tta}")

    views = [(1.0, False)]
    if args.tta:
        views += [(1.0, True), (0.85, False), (0.85, True)]

    def predict_probs(image_path):
        probs = []
        for crop_scale, flip in views:
            tensor = classifier._preprocess(
                str(image_path), crop_scale=crop_scale, flip=flip)
            if tensor is None:
                continue
            with torch.inference_mode():
                logits = classifier.model(tensor.to(device))
                probs.append(
                    torch.softmax(logits[0], dim=0).cpu().numpy())
        if not probs:
            return None
        return np.mean(np.stack(probs), axis=0)

    per_class = defaultdict(lambda: {"n": 0, "correct": 0, "top3": 0})
    confusion = defaultdict(Counter)
    failed = 0
    top1_hits = top3_hits = 0

    for i, row in enumerate(rows, 1):
        image_path = raw_root / row["path"]
        probs = predict_probs(image_path)
        if probs is None:
            failed += 1
            continue

        order = np.argsort(probs)[::-1]
        top1_idx = int(order[0])
        top3_idx = set(int(x) for x in order[:3])

        true_idx = row["label"]
        true_name = class_names[true_idx]
        pred_name = class_names[top1_idx]

        per_class[true_name]["n"] += 1
        confusion[true_name][pred_name] += 1
        if top1_idx == true_idx:
            top1_hits += 1
            per_class[true_name]["correct"] += 1
        if true_idx in top3_idx:
            top3_hits += 1
            per_class[true_name]["top3"] += 1

        if i % 50 == 0:
            print(f"  已评估 {i}/{len(rows)}")

    n = sum(v["n"] for v in per_class.values())
    report = {
        "checkpoint": str(checkpoint),
        "manifest": str(args.manifest),
        "image_size": args.image_size,
        "device": str(device),
        "tta": args.tta,
        "n": n,
        "failed": failed,
        "top1": round(top1_hits / n, 4) if n else None,
        "top3": round(top3_hits / n, 4) if n else None,
        "per_class": {
            name: {
                "n": v["n"],
                "correct": v["correct"],
                "recall": round(v["correct"] / v["n"], 4) if v["n"] else None,
                "top3": round(v["top3"] / v["n"], 4) if v["n"] else None,
            }
            for name, v in sorted(per_class.items())
        },
        "confusion": {
            true_name: dict(counter.most_common())
            for true_name, counter in sorted(confusion.items())
        },
    }
    # macro recall 只看每个类别的召回率，不受类别数量不平衡影响。
    recalls = [v["recall"] for v in report["per_class"].values()
               if v["recall"] is not None]
    report["macro_recall"] = round(float(np.mean(recalls)), 4) if recalls else None

    print(json.dumps({
        "n": report["n"],
        "failed": report["failed"],
        "top1": report["top1"],
        "top3": report["top3"],
        "macro_recall": report["macro_recall"],
    }, ensure_ascii=False, indent=2))
    print("\n每类召回率：")
    for name, v in report["per_class"].items():
        print(f"  {name:28s} n={v['n']:3d} recall={v['recall']:.3f} top3={v['top3']:.3f}")

    if args.out:
        out = Path(args.out)
        out.parent.mkdir(parents=True, exist_ok=True)
        with open(out, "w", encoding="utf-8") as fh:
            json.dump(report, fh, ensure_ascii=False, indent=2)
        print(f"\n完整报告: {out}")


if __name__ == "__main__":
    main()
