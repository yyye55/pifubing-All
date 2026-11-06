# 问诊风格评测

这个目录只用于本地评估，不参与线上运行，也不会被 FastAPI 自动加载。

## 跑法

1. 启动 Python 服务（默认 `http://127.0.0.1:8001`）。
2. 先跑少量用例确认服务正常：

```bash
python eval/run_eval.py --limit 5
```

3. 按类别跑：

```bash
python eval/run_eval.py --category image --out eval/results_image.jsonl
```

4. 全部跑完后，按 `rubric.md` 人工盲评；`results.jsonl` 里的 `checks` 只是硬性规则检查，不代表语义质量。

## 注意

- 用例里没有真实用户数据，全部是通用皮肤问题。
- 脚本只调用现有 `/process_text_stream`，不写数据库、不写记忆。
- 流式接口回答后会额外生成追问，因此单条可能多等几秒；批量跑时可适当调大 `--timeout`。
