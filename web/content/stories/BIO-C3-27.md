---
id: BIO-C3-27
title: 飞升焦点里的分子涨落自相关：马格德、埃尔森与荧光相关光谱（FCS）
lab_item: 亚飞升（<1 fL）微聚焦共聚焦激光激发光腰、单光子雪崩二极管（SPAD）与硬件自相关仪
year: 1972
month: 9
day: 11
tags:
  - 荧光相关光谱
  - 热力学涨落
  - 扩散动力学
summary: 道格拉斯·马格德与瓦特·韦伯通过监测微共焦体积内荧光强度的自发热力学涨落，利用自相关运算解算出分子的布朗扩散系数与结合态。
geek_quote: 庸人千方百计抚平信号里的毛刺噪声，物理学家却在纳秒与微秒的微澜中听到了布朗运动的足音；在方寸飞升的虚空舞台上，分子的每一次出场都被谱写成了自相关的乐章。
source:
  type: paper
  citation: 'Magde, D., Elson, E., & Webb, W. W. (1972). Thermodynamic fluctuations in a chemical system—measurement by fluorescence correlation spectroscopy. Physical Review Letters, 29(11), 705-708.'
  url: 'https://doi.org/10.1103/PhysRevLett.29.705'
---
在大众直觉中，实验数据里的“波动与噪声”向来是需要被滤镜和平均化彻底消灭的敌人。但在统计物理学家眼中，涨落恰恰蕴藏着热力学最深刻的本质——正如昂萨格倒易关系所揭示的，系统自发热涨落的衰减规律与宏观偏离平衡态的弛豫规律完全同构。1972年，康奈尔大学的道格拉斯·马格德（Douglas Magde）、埃利奥特·埃尔森（Elliot Elson）与瓦特·韦伯（Watt W. Webb）开创了荧光相关光谱学（Fluorescence Correlation Spectroscopy, FCS）。他们用高数值孔径物镜将激光极致约束在一个体积不足一飞升（$10^{-15}$升）的微型共焦椭球体内，当极稀薄的荧光分子由于热运动布朗扩散进出该体积时，高灵敏单光子探测器会捕捉到荧光光强剧烈的剧烈尖峰与波谷涨落。硬件自相关仪随后对光强时间序列计算自相关函数 $G(	au) = \langle \delta F(t) \cdot \delta F(t+	au) angle / \langle F angle^2$。
