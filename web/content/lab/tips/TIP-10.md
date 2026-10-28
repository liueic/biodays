---
id: TIP-10
section: tips
topic: 质控排错
title: 流式细胞术中未标记血清或纯化抗体预阻断 Fc 受体消除假阳性
lab_item: 同源种属正常血清（如正常小鼠/大鼠血清）或抗 CD16/CD32 单克隆抗体（Fc Block）、流式染色缓冲液（PBS + 1% BSA）
tags:
  - 流式细胞术
  - Fc受体阻断
  - 假阳性控制
  - 抗体染色
summary: 在脾脏、骨髓、外周血单核细胞（PBMC）或巨噬细胞流式染色中，荧光抗体经常出现全群细胞弥散偏移的“假阳性阳性”。在加入特异性一抗前加入同源正常血清或抗 CD16/32 预温育，彻底阻断 Fc 受体非特异结合。
geek_quote: 抗体的后半身（Fc 段）和它的前半身（Fab 端）一样贪婪；若不管好它的尾巴，它就会抓住迎面走来的任何一个吞噬细胞。
source:
  type: paper
  citation: 'Andersen, M. N., et al. (2016). Elimination of erroneous results in flow cytometry caused by non-specific binding of monoclonal antibodies. Journal of Immunological Methods, 436, 45–51.'
  url: 'https://doi.org/10.1016/j.jim.2016.06.007'
---

### 背景与痛点

流式细胞术（FACS）分析免疫细胞亚群时，初学者最常犯的致命错误就是“跳过 Fc 阻断”。当把荧光偶联抗体直接加入含有单核细胞（Monocytes）、巨噬细胞（Macrophages）、树突状细胞（DC）、B 细胞或中性粒细胞的样本悬液中时，常常发现阴性对照管（Isotype Control）或未表达目标抗原的细胞群整群向上漂移，出现高达 $30\%\sim 70\%$ 的虚假假阳性结合信号。

### 巧思与免疫球蛋白构象机制

2016 年 Andersen 等人在 *Journal of Immunological Methods* 深入揭示了单克隆抗体在流式分析中的非特异结合根源：

1. **Fc 受体的高亲和力捕获**：免疫细胞表面广泛表达高密度的 **Fc 受体（Fc Receptors, FcR）**，包括小鼠的 $\text{Fc}\gamma\text{RI}$（CD64）、$\text{Fc}\gamma\text{RIIB}$（CD32）、$\text{Fc}\gamma\text{RIII}$（CD16）或人类同源受体。抗体分子由抗原特异识别区（Fab）和恒定区结晶片段（Fc）组成。Fc 受体的天然生理使命就是识别并结合外来 IgG 分子的 Fc 骨架。因此，即便你的荧光标记抗体 Fab 端并不针对巨噬细胞，其 Fc 段依然会被巨噬细胞表面的 CD16/32 贪婪结合，造成强烈荧光信号。
2. **两阶段竞争性饱和阻断（Fc Blocking）**：消除这一假象的核心逻辑不是削弱荧光，而是通过在正式染色前引入高浓度的**非特异性无荧光饱和剂**（如纯化未标记的抗 CD16/32 单克隆抗体，或同源物种的正常未免疫血清）。让这些无害的抗体先行将细胞表面的全部 Fc 受体结合口袋死死封堵占满；随后加入的荧光特异性单抗便只能通过自身的 Fab 端与目标膜抗原表位进行真正的特异性结合。

### 阻断剂选型与实操 SOP

#### 方案 A：针对小鼠样品的抗 CD16/CD32 单抗（最严谨方案）
1. 取重悬于流式染色缓冲液（FACS Buffer: $1\times\text{PBS} + 1\%\ \text{BSA} + 0.05\%\ \text{NaN}_3$）中的单细胞悬液（约 $1\times 10^6$ 个细胞 / $100\ \mu\text{L}$）。
2. 每管直接加入 **$0.5\sim 1.0\ \mu\text{g}$（通常约 $0.5\sim 1.0\ \mu\text{L}$）的纯化未标记大鼠抗小鼠 CD16/CD32 单抗（Clone 2.4G2）**。
3. 移液枪轻柔吹打混匀，置于 **$4^\circ\text{C}$（或冰上）避光孵育 $10\sim 15$ 分钟**。
4. **关键细节：无需清洗！** 孵育结束后直接向管内加入后续的目标荧光一抗混悬液，继续冰上避光染色 30 分钟。

#### 方案 B：针对人 PBMC 或大鼠样本的正常血清法（经济实用方案）
1. 人外周血单核细胞可直接使用 **同源正常人 AB 型血清（Human AB Serum）** 或 **正常山羊/兔血清**。
2. 在配制流式抗体主稀释液时，直接向 FACS Buffer 中加入 **$5\%\sim 10\%\ (\text{v/v})$ 的热灭活正常血清**。
3. 血清中海量的正常无荧光多克隆 IgG 分子会瞬间将全部表面 Fc 受体结合饱和，背景信噪比提升数倍。
