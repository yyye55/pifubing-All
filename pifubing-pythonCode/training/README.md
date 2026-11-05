# 15 类皮肤病分类训练流程

本目录只做训练和评估，不修改线上服务、数据库和后端接口。
模型结构与 `../test.py` 完全一致，训练产物可以直接替换 `B3_Transformer.pth`。

## 1. 环境

- Python 3.10.11，虚拟环境：`training/.venv`
- torch 2.14.1+cu126，torchvision 0.29.1+cu126
- 显卡：RTX 4060 Laptop 8 GB（CPU 也能跑，但慢很多）

```bash
training/.venv/Scripts/python.exe -c "import torch; print(torch.__version__, torch.cuda.is_available())"
```

## 2. 数据

原始数据：`D:\皮肤病相关资料\pifubing-data\skin_data`
（`train/`、`test/` 两个目录，26 个类别文件夹）

只取与线上模型一致的 15 个类别，类别顺序以 `test.py` 的
`DEFAULT_CLASS_NAMES` 为准，映射写在 `prepare_dataset.py` 的 `CLASSES` 里。

## 3. 三步流程

### 第一步：清洗并生成清单

```bash
training/.venv/Scripts/python.exe training/prepare_dataset.py
```

- 只读原始图片，不复制、不改名。
- 用 MD5 找完全重复的图片。
- 训练集里与测试集重复的图片删除，避免数据泄漏。
- 同一张图对应两个类别时整组丢弃。
- 从清洗后的训练集按类别分层切出 15% 做验证集。
- 输出到 `D:\皮肤病相关资料\pifubing-data\manifests_15`：
  `train.csv`、`val.csv`、`test.csv`、`test_strict.csv`、
  `classes.json`、`stats.json`、`removed.csv`。

本次结果：18,278 张目标图片，删除 1,273 张（重复/标签冲突），
最终 train 11,325 / val 1,997 / test 3,683。
清洗后跨集重复 0、标签冲突 0。

`test_strict.csv` 是“旧模型训练时没见过”的 3,210 张测试图，
用来公平比较新旧模型；`test.csv` 是全部 3,683 张。

### 第二步：训练

```bash
training/.venv/Scripts/python.exe -u training/train.py \
  --init B3_Transformer.pth --epochs 12 --batch-size 16 \
  --accum-steps 1 --image-size 300 --lr 1e-4 --backbone-lr 1e-5 \
  --weight-decay 1e-4 --freeze-blocks 5 --patience 4 \
  --monitor macro_f1 --num-workers 0 --seed 42 \
  --out-dir training/runs/finetune_clean_v1
```

- `--init`：从现有权重微调；改用 `--scratch` 则从 ImageNet 主干重训。
- `--freeze-blocks 5`：冻结 CNN 前 5 个 block，只训练后部卷积、
  Transformer 和分类头。
- 损失函数：按类别频率加权的 Focal Loss。
- 按验证集 macro-F1 保存 `best.pth`，并开启早停。
- 输出：`best.pth`、`last.pth`、`best_meta.json`、`history.csv`、`classes.json`。

Windows 上 `--num-workers` 必须为 0；实测大于 0 会报
`LLVM ERROR: out of memory`。

### 第三步：评估

```bash
training/.venv/Scripts/python.exe training/evaluate.py \
  --checkpoint training/runs/finetune_clean_v1/best.pth \
  --manifest "D:/皮肤病相关资料/pifubing-data/manifests_15/test.csv" \
  --per-class 0 --device cuda --tta \
  --out training/runs/finetune_clean_v1/eval_test_tta.json
```

- `--per-class 0` 表示跑全部图片，不带此参数默认每类抽 20 张。
- `--tta` 开启 4 视图测试时增强（原图、翻转、轻微放大、放大后翻转）。

## 4. 本次结果（2026-11-05）

| 测试集 | TTA | Top-1 | Top-3 | Macro Recall |
|---|---|---|---|---|
| 全量测试 3,683 张 | 否 | 83.03% | 95.66% | 84.93% |
| 全量测试 3,683 张 | 是 | 84.39% | 96.23% | 86.29% |
| 严格测试 3,210 张 | 否 | 82.93% | 95.42% | 85.96% |
| 严格测试 3,210 张 | 是 | 84.49% | 96.04% | 87.23% |

旧模型基线（全量测试）：Top-1 81.18%、Top-3 94.71%、Macro 83.83%。

明显改善的类别：湿疹 64.4%→75.2%、银屑病 64.3%→70.8%、
体癣 72.1%→77.3%、恶性痣 88.3%→94.3%。
有回退的类别：疱疹/HPV/STD 88.4%→77.9%、良性痣 89.7%→83.3%。

## 5. 权重替换与回滚

本次已将 `training/runs/finetune_clean_v1/best.pth` 复制为
`B3_Transformer.pth`（两者 MD5 一致）。旧权重备份在仓库外：

```
D:\皮肤病相关资料\pifubing-data\models\B3_Transformer_old_2025-02-09.pth
```

回滚：

```bash
cp "D:/皮肤病相关资料/pifubing-data/models/B3_Transformer_old_2025-02-09.pth" B3_Transformer.pth
```

注意：正在运行的 Python 服务会缓存已加载的模型，替换文件后需要重启服务
才会生效；本次没有重启或改动任何服务器。

## 6. 线上开启 TTA

`test.py` 支持环境变量 `ENABLE_TTA=1`，默认关闭。
本次实测 TTA 带来约 +1.4 个百分点 Top-1，但推理耗时约为原来的 4 倍。
CPU 部署建议先压测延迟，再决定是否开启。

## 7. 后续可做

1. 对“疱疹/HPV/STD”“良性痣”做针对性分析，必要时补充样本或调整采样。
2. 置信度校准 + 拒识：低置信度时输出“建议就医/无法判断”，而不是强行给类别。
3. 扩展到全部 26 类（含 Heathy），同时更新知识库和前端类别。
4. 分割、严重程度回归、OOD 检测属于后续独立任务。
