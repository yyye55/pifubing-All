# -*- coding: utf-8 -*-
"""为 15 类皮肤分类任务生成干净的数据清单。

做的事情：
1. 只读取原始数据，不修改、不复制原始图片。
2. 用 MD5 找完全重复的图片。
3. 训练集里如果出现和测试集完全相同的图片，删掉训练集那一份，避免数据泄漏。
4. 同一张图被放进两个不同类别时，视为标签冲突，整组丢弃。
5. 从清洗后的训练集里分层切出 15% 做验证集。
6. 输出 train/val/test 三个 CSV 清单、classes.json、stats.json、
   removed.csv（被删文件及原因），方便复查。

用法：
    python training/prepare_dataset.py
可覆盖的环境变量：
    SKIN_RAW_ROOT       原始数据目录，默认 D:/皮肤病相关资料/pifubing-data/skin_data
    SKIN_MANIFEST_DIR   清单输出目录，默认 D:/皮肤病相关资料/pifubing-data/manifests_15
    SKIN_VAL_RATIO      验证集比例，默认 0.15
    SKIN_SEED           随机种子，默认 42
"""

import csv
import hashlib
import json
import os
import random
import sys
from collections import Counter, defaultdict
from pathlib import Path

from PIL import Image

# 类别顺序必须和现有模型的输出顺序一致（test.py 里的 DEFAULT_CLASS_NAMES）。
# 这个顺序已经用小样本验证过：现有权重在这份数据上的 top-1 约为 80%。
CLASSES = [
    (0, "痤疮和玫瑰痤疮", "Acne and Rosacea Photos"),
    (1, "光化性角化病、基底细胞癌及其他皮肤癌",
     "Actinic Keratosis Basal Cell Carcinoma and other Malignant Lesions"),
    (2, "特应性皮炎", "Atopic Dermatitis Photos"),
    (3, "良性痣", "benign_Moles"),
    (4, "湿疹", "Eczema Photos"),
    (5, "发疹和药物疹", "Exanthems and Drug Eruptions"),
    (6, "疱疹、人乳头瘤病毒及其他性传播疾病", "Herpes HPV and other STDs Photos"),
    (7, "光照性疾病和色素沉着障碍", "Light Diseases and Disorders of Pigmentation"),
    (8, "恶性痣", "malignant_Moles"),
    (9, "甲真菌病及其他指甲疾病", "Nail Fungus and other Nail Disease"),
    (10, "银屑病、扁平苔藓及相关疾病",
     "Psoriasis pictures Lichen Planus and related diseases"),
    (11, "脂溢性角化病及其他良性肿瘤",
     "Seborrheic Keratoses and other Benign Tumors"),
    (12, "全身性疾病", "Systemic Disease"),
    (13, "体癣、念珠菌病及其他真菌感染",
     "Tinea Ringworm Candidiasis and other Fungal Infections"),
    (14, "疣、传染性软疣及其他病毒感染",
     "Warts Molluscum and other Viral Infections"),
]

RAW_ROOT = Path(os.getenv(
    "SKIN_RAW_ROOT", r"D:\皮肤病相关资料\pifubing-data\skin_data"))
OUT_DIR = Path(os.getenv(
    "SKIN_MANIFEST_DIR", r"D:\皮肤病相关资料\pifubing-data\manifests_15"))
VAL_RATIO = float(os.getenv("SKIN_VAL_RATIO", "0.15"))
SEED = int(os.getenv("SKIN_SEED", "42"))
IMAGE_EXTS = {".jpg", ".jpeg", ".png", ".bmp", ".webp"}

FOLDER_TO_CLASS = {folder: (idx, zh) for idx, zh, folder in CLASSES}


def relative(path: Path) -> str:
    """清单里保存相对路径，换机器时只需要改 RAW_ROOT。"""
    return path.relative_to(RAW_ROOT).as_posix()


def scan_records():
    """扫描 15 个目标类别，返回记录列表。"""
    records = []
    for split in ("train", "test"):
        split_dir = RAW_ROOT / split
        if not split_dir.is_dir():
            raise FileNotFoundError(f"找不到数据目录: {split_dir}")
        for folder, (label, zh) in FOLDER_TO_CLASS.items():
            class_dir = split_dir / folder
            if not class_dir.is_dir():
                print(f"[警告] 缺少类别目录: {class_dir}")
                continue
            for path in class_dir.iterdir():
                if not path.is_file() or path.suffix.lower() not in IMAGE_EXTS:
                    continue
                records.append({
                    "id": len(records),
                    "split": split,
                    "label": label,
                    "zh": zh,
                    "folder": folder,
                    "path": path,
                    "size": path.stat().st_size,
                })
    return records


def validate_images(records):
    """用 Pillow 检查图片能否打开，去掉损坏图和过小的图。"""
    valid, invalid = [], []
    for i, r in enumerate(records, 1):
        try:
            with Image.open(r["path"]) as im:
                width, height = im.size
                im.verify()
            if width < 32 or height < 32:
                invalid.append((r, f"too_small:{width}x{height}"))
            else:
                r["width"], r["height"] = width, height
                valid.append(r)
        except Exception as exc:
            invalid.append((r, f"invalid_image:{type(exc).__name__}"))
        if i % 2000 == 0:
            print(f"  已检查 {i}/{len(records)} 张图片")
    return valid, invalid


def hash_size_candidates(records):
    """只对“文件大小相同”的图片算 MD5，足以找出完全重复的图片。"""
    by_size = defaultdict(list)
    for r in records:
        by_size[r["size"]].append(r)

    candidates = [group for group in by_size.values() if len(group) > 1]
    print(f"  文件大小相同的候选组: {len(candidates)}，"
          f"涉及图片: {sum(len(g) for g in candidates)} 张")

    by_hash = defaultdict(list)
    done = 0
    for group in candidates:
        for r in group:
            h = hashlib.md5()
            with open(r["path"], "rb") as fh:
                for chunk in iter(lambda: fh.read(1024 * 1024), b""):
                    h.update(chunk)
            r["md5"] = h.hexdigest()
            by_hash[r["md5"]].append(r)
            done += 1
            if done % 2000 == 0:
                print(f"  已计算 {done} 张候选图片的 MD5")
    return by_hash


def deduplicate(records, by_hash):
    """返回保留记录的 id 集合，以及删除原因列表。

    规则优先级：测试集优先保留；训练集与测试集重复时删训练集；
    同一 hash 出现多个类别时整组丢弃；同类别重复只保留一张。
    """
    keep_ids = {r["id"] for r in records}
    removed = []

    def drop(r, reason):
        keep_ids.discard(r["id"])
        removed.append({
            "path": relative(r["path"]),
            "split": r["split"],
            "label": r["label"],
            "zh": r["zh"],
            "reason": reason,
        })

    duplicate_groups = 0
    for group in by_hash.values():
        if len(group) < 2:
            continue
        duplicate_groups += 1
        tests = [r for r in group if r["split"] == "test"]
        trains = [r for r in group if r["split"] == "train"]

        if tests:
            test_labels = {r["label"] for r in tests}
            if len(test_labels) > 1:
                # 同一张图在测试集里被标成不同类别，测试标签不可信，整组丢弃。
                for r in group:
                    drop(r, "test_label_conflict")
                continue

            # 测试集优先保留一张，其余测试重复删掉。
            keeper = sorted(tests, key=lambda r: relative(r["path"]))[0]
            for r in tests:
                if r["id"] != keeper["id"]:
                    drop(r, "test_duplicate")
            # 训练集里与测试集重复的副本全部删掉，避免泄漏。
            for r in trains:
                drop(r, "train_test_duplicate")
            continue

        train_labels = {r["label"] for r in trains}
        if len(train_labels) > 1:
            # 同一张图被放进两个训练类别，标签本身有冲突，整组丢弃。
            for r in group:
                drop(r, "train_label_conflict")
            continue

        keeper = sorted(trains, key=lambda r: relative(r["path"]))[0]
        for r in trains:
            if r["id"] != keeper["id"]:
                drop(r, "train_duplicate")

    return keep_ids, removed, duplicate_groups


def stratified_split(train_records):
    """按类别分层切出验证集，返回 (train_ids, val_ids)。"""
    by_label = defaultdict(list)
    for r in train_records:
        by_label[r["label"]].append(r)

    train_ids, val_ids = set(), set()
    for label, items in sorted(by_label.items()):
        items = sorted(items, key=lambda r: relative(r["path"]))
        random.Random(SEED + label).shuffle(items)
        n_val = max(1, int(round(len(items) * VAL_RATIO)))
        n_val = min(n_val, len(items) - 1)
        for r in items[:n_val]:
            val_ids.add(r["id"])
        for r in items[n_val:]:
            train_ids.add(r["id"])
    return train_ids, val_ids


def write_manifests(records, train_ids, val_ids, keep_ids, invalid, removed,
                    raw_train_hashes):
    OUT_DIR.mkdir(parents=True, exist_ok=True)

    final_split = {}
    for r in records:
        if r["id"] not in keep_ids:
            # 被去重/冲突规则删掉的图片，不能进入任何清单。
            continue
        if r["id"] in train_ids:
            final_split[r["id"]] = "train"
        elif r["id"] in val_ids:
            final_split[r["id"]] = "val"
        elif r["split"] == "test":
            final_split[r["id"]] = "test"

    rows = [r for r in records if r["id"] in final_split]
    for r in rows:
        r["final_split"] = final_split[r["id"]]
        # 旧模型训练时见过这张测试图的原始副本，评估旧模型时要单独标记，
        # 否则旧模型的分数会被“记忆”抬高。
        r["seen_in_raw_train"] = int(
            r["final_split"] == "test" and r.get("md5") in raw_train_hashes)

    header = ["path", "label", "zh", "class_folder", "seen_in_raw_train"]
    for split in ("train", "val", "test"):
        out = OUT_DIR / f"{split}.csv"
        with open(out, "w", newline="", encoding="utf-8-sig") as fh:
            writer = csv.writer(fh)
            writer.writerow(header)
            for r in sorted(rows, key=lambda x: (x["label"], relative(x["path"]))):
                if r["final_split"] == split:
                    writer.writerow([
                        relative(r["path"]), r["label"], r["zh"], r["folder"],
                        r["seen_in_raw_train"],
                    ])

    # 严格测试集：只保留“原始训练集里从未出现过”的测试图，
    # 用来公平比较旧模型和新模型。
    with open(OUT_DIR / "test_strict.csv", "w", newline="",
              encoding="utf-8-sig") as fh:
        writer = csv.writer(fh)
        writer.writerow(header)
        for r in sorted(rows, key=lambda x: (x["label"], relative(x["path"]))):
            if r["final_split"] == "test" and not r["seen_in_raw_train"]:
                writer.writerow([
                    relative(r["path"]), r["label"], r["zh"], r["folder"], 0,
                ])

    with open(OUT_DIR / "classes.json", "w", encoding="utf-8") as fh:
        json.dump([
            {"index": idx, "zh": zh, "folder": folder}
            for idx, zh, folder in CLASSES
        ], fh, ensure_ascii=False, indent=2)

    all_removed = removed + [{
        "path": relative(r["path"]),
        "split": r["split"],
        "label": r["label"],
        "zh": r["zh"],
        "reason": reason,
    } for r, reason in invalid]
    with open(OUT_DIR / "removed.csv", "w", newline="", encoding="utf-8-sig") as fh:
        writer = csv.DictWriter(
            fh, fieldnames=["path", "split", "label", "zh", "reason"])
        writer.writeheader()
        writer.writerows(sorted(all_removed, key=lambda x: x["path"]))

    def counts(rs):
        c = Counter((r["final_split"], r["label"]) for r in rs)
        return {
            split: {
                zh: c[(split, idx)]
                for idx, zh, _ in CLASSES
            }
            for split in ("train", "val", "test")
        }

    # 清洗后的自检：同一 MD5 不允许同时出现在 train 和 test，
    # 也不允许同一张图对应两个不同类别。
    hash_splits = defaultdict(set)
    hash_labels = defaultdict(set)
    for r in rows:
        if r.get("md5"):
            hash_splits[r["md5"]].add(r["final_split"])
            hash_labels[r["md5"]].add(r["label"])
    cross_split_after = sum(
        1 for splits in hash_splits.values()
        if "train" in splits and "test" in splits
    )
    label_conflicts_after = sum(
        1 for labels in hash_labels.values() if len(labels) > 1
    )

    stats = {
        "raw_root": str(RAW_ROOT),
        "manifest_dir": str(OUT_DIR),
        "classes": len(CLASSES),
        "original_total": len(records) + len(invalid),
        "invalid_images": len(invalid),
        "removed_total": len(all_removed),
        "removed_reasons": dict(Counter(x["reason"] for x in all_removed)),
        "cross_split_duplicates_after_clean": cross_split_after,
        "label_conflicts_after_clean": label_conflicts_after,
        "test_seen_in_raw_train": sum(
            1 for r in rows
            if r["final_split"] == "test" and r["seen_in_raw_train"]),
        "test_strict_total": sum(
            1 for r in rows
            if r["final_split"] == "test" and not r["seen_in_raw_train"]),
        "final_counts": counts(rows),
        "final_total": len(rows),
    }
    with open(OUT_DIR / "stats.json", "w", encoding="utf-8") as fh:
        json.dump(stats, fh, ensure_ascii=False, indent=2)
    return stats


def main():
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8")

    print(f"原始数据: {RAW_ROOT}")
    print(f"输出目录: {OUT_DIR}")
    records = scan_records()
    print(f"扫描到 {len(records)} 张目标类别图片")

    print("检查图片完整性...")
    records, invalid = validate_images(records)
    print(f"有效图片 {len(records)} 张，损坏/过小 {len(invalid)} 张")

    print("检测重复图片...")
    by_hash = hash_size_candidates(records)
    keep_ids, removed, duplicate_groups = deduplicate(records, by_hash)
    print(f"重复组 {duplicate_groups} 个，删除 {len(removed)} 张重复/冲突图片")

    kept = [r for r in records if r["id"] in keep_ids]
    train_records = [r for r in kept if r["split"] == "train"]
    train_ids, val_ids = stratified_split(train_records)
    print(f"清洗后训练集 {len(train_records)} 张，"
          f"切分 train={len(train_ids)} / val={len(val_ids)}")

    raw_train_hashes = {
        r["md5"] for r in records
        if r["split"] == "train" and r.get("md5")
    }
    stats = write_manifests(
        records, train_ids, val_ids, keep_ids, invalid, removed,
        raw_train_hashes)
    print(json.dumps(stats, ensure_ascii=False, indent=2))
    print(f"\n完成，清单已写入: {OUT_DIR}")


if __name__ == "__main__":
    main()
