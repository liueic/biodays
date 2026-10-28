---
id: PROT-05
section: protocols
topic: 操作流程
title: Tris-Tricine-SDS-PAGE 体系分离解析极小分子量多肽（1~20 kDa）
lab_item: Tricine（三(羟甲基)甲基甘氨酸）、Tris 碱、10× 阳极缓冲液、10× 阴极缓冲液、垂直电泳槽
tags:
  - 小分子多肽
  - Tris-Tricine
  - SDS-PAGE
  - 蛋白质分离
summary: 经典 Laemmli（Tris-甘氨酸）电泳在检测小于 10 kDa 的小肽、抗微生物多肽或切割片段时，小肽条带常被前端高浓度 SDS 胶束掩盖成一片模糊宽带。采用 Tricine 作为尾随离子，在 16.5% 凝胶中实现 1~20 kDa 多肽的锋利极窄聚焦。
geek_quote: 甘氨酸由于泳动太慢挡住了小肽的前路；换上轻盈敏捷的 Tricine，小多肽才能跑出独立的风采。
source:
  type: paper
  citation: 'Schägger, H., & von Jagow, G. (1987). Tricine-sodium dodecyl sulfate-polyacrylamide gel electrophoresis for the separation of proteins in the range from 1 to 100 kDa. Analytical Biochemistry, 166(2), 368–379.'
  url: 'https://doi.org/10.1016/0003-2697(87)90587-2'
---

### 背景与痛点

在分析合成多肽（Peptides）、抗菌肽（AMPs）、泛素多聚物切割片段或分泌性小细胞因子（<15 kDa）时，如果继续沿用教材经典的 Laemmli（Tris-Glycine）SDS-PAGE 体系，会观察到一种不可抗拒的物理缺陷：在低交联度的普通凝胶中，小于 10 kDa 的多肽迁移速度极快，它们在浓缩胶与分离胶交界面上无法被有效压缩聚焦；更致命的是，多肽会直接撞入自由游动的带负电 SDS 胶束峰（Micelle front），小多肽与前沿胶束混为一体，导致整个胶底一片模糊的黑晕，分辨率完全丧失。

### 巧思与离子淌度动力学机制

1987 年 Hermann Schägger 与 Gebhard von Jagow 在 *Analytical Biochemistry* 发表了生化史上最高引用的技术文献之一，奠定了 **Tris-Tricine 电泳系统**：

1. **迁移率更高的尾随离子**：在 Laemmli 系统中，甘氨酸（Glycine）在酸性浓缩胶中的电离度极低，有效有效淌度很小；而 Tricine（$N$-\[三(羟甲基)甲基]甘氨酸）具有更低的酸度系数（$\text{p}K_a \approx 8.15$，相比甘氨酸的 $9.6$）。在相同 pH 条件下，Tricine 的净负电荷明显高于甘氨酸，因此具有快得多的有效迁移速度。
2. **小多肽在先导与尾随离子间窄缝聚焦**：在分离胶中，先导离子（$\text{Cl}^-$）泳动最快，尾随离子（$\text{Tricine}^-$）紧随其后。由于 Tricine 迁移迅速，它与氯离子之间的“同向电泳压强”将所有 $1\sim 20\ \text{kDa}$ 的小分子多肽死死压缩在一道极窄的高密度分子束中，使其脱离前方游离的 SDS 胶束区，并在后续进入分离胶时获得极其清晰锐利的独立条带。

### 电泳缓冲液配制规范

#### 1. 阳极缓冲液（Anode Buffer, 10×，装入下槽）
- $1.0\ \text{M}\ \text{Tris}$ 碱（$121.1\ \text{g/L}$），加入浓盐酸（$\text{HCl}$）调至 **$\text{pH} = 8.9$**。
- 使用时以去离子水稀释 10 倍注入外层/底部阳极槽。

#### 2. 阴极缓冲液（Cathode Buffer, 10×，装入上槽）
- $1.0\ \text{M}\ \text{Tris}$ 碱（$121.1\ \text{g/L}$）
- $1.0\ \text{M}\ \text{Tricine}$（$179.2\ \text{g/L}$）
- $1.0\%\ (\text{w/v})\ \text{SDS}$（$10\ \text{g/L}$）
- 溶液天然 $\text{pH}$ 约为 $8.25$，**严禁加酸碱调节！**
- 使用时以去离子水稀释 10 倍注入内层阴极加样槽。

#### 3. 凝胶缓冲液（Gel Buffer, 3×）
- $3.0\ \text{M}\ \text{Tris}$ 碱 $+ 0.3\%\ \text{SDS}$，调 $\text{pH} = 8.45$。

### 16.5% 高分辨分离胶制备（针对 1~10 kDa 小肽）

- $49.5\%\ \text{T} / 3\%\ \text{C}$ 丙烯酰胺/甲叉双丙烯酰胺单体母液：$3.3\ \text{mL}$
- $3\times\text{Gel Buffer}$：$3.3\ \text{mL}$
- 纯甘油（Glycerol）：$1.0\ \text{g}$（加入甘油可增加凝胶内摩擦阻力，显著提高 $1\sim 5\ \text{kDa}$ 超小肽的机械截留分辨）
- 超纯水补足至 $10\ \text{mL}$，加 $50\ \mu\text{L}\ 10\%\ \text{APS}$ 与 $5\ \mu\text{L}\ \text{TEMED}$ 灌胶聚合。

### 电泳参数建议
- 样品上样前必须加入 Tricine 专用还原型上样缓冲液并在 $95^\circ\text{C}$ 充分变性 5 分钟。
- **恒压模式**：初始以 **$30\ \text{V}$** 电泳约 30 分钟使样品平缓进入浓缩胶；当染料前沿跨过界线进入分离胶后，调高电压至 **$90\sim 100\ \text{V}$** 恒压电泳直至溴酚蓝前沿到达凝胶最底端。
