# 故事数据

共 408 篇，原始内容和 ID 保持不变，均从既有数据源复制。

| 文件 | 篇数 | 来源文件 |
| --- | ---: | --- |
| `stories_q1.json` 至 `stories_q4.json` | 各 52，共 208 | 原始季度汇总 |
| `stories_cluster_1_biochem_metabolism.json` | 50 | `raw_sources/cluster_1_biochem_metabolism.json` |
| `stories_cluster_2_cloning_genetics.json` | 50 | `raw_sources/cluster_2_cloning_genetics.json` |
| `stories_cluster_3_instruments_biophysics.json` | 50 | `raw_sources/cluster_3_instruments_biophysics.json` |
| `stories_cluster_4_immunology_microbiology.json` | 50 | `raw_sources/cluster_4_immunology_microbiology.json` |

主题文件添加 `stories_` 前缀以供现有导入器识别，内容与来源文件逐字节一致。原目录中的 `q2_p*.json`、`q3_p*.json` 是季度汇总的拆分副本，记录和内容已包含在季度文件中，不重复导入；`gen_*.py` 是生成脚本，不作为故事数据分发。

JSON 是数据源，SQLite 是构建产物，不提交数据库、WAL 或 SHM 文件。修改或补充故事后，运行 `cd server && go test ./internal/seed` 校验实际数据，再重建容器镜像。构建使用 `biodays seed --strict`，重复 ID、无效记录、损坏文件或空数据会令构建失败。

故事 ID 不必采用 `Q1-01` 格式；`BIO-C1-01` 或其他稳定唯一 ID（最多 16 字节）均参与归档和推荐。日期推荐只读取 `month` / `day`，改正文时保留原 ID，新增故事使用新 ID。持续同步和更新清单的命令见[根目录说明](../README.md#持续更新故事)。
