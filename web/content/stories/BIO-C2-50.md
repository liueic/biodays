---
id: BIO-C2-50
title: 给狂奔的核糖体踩下急刹：英戈利亚与魏斯曼发明 Ribo-seq 翻译组快照
lab_item: 环己酰亚胺（Cycloheximide 翻译阻断剂）、大肠杆菌 RNase I 核酸酶、蔗糖密度梯度超速离心管与高通量建库磁珠
year: 2009
month: 4
day: 10
tags:
  - Ribo-seq
  - 核糖体印迹
  - 翻译组学
summary: RNA-seq只能测量转录本存在无法证实其是否翻译；Ribo-seq利用核糖体足迹保护酶解，以单密码子分辨率透视全细胞翻译实况。
geek_quote: 生命最壮丽的流水线，不需要去库房盘点堆积的图纸，只需在车间突然拉响警报，抓拍工人们手上紧握的零件。
source:
  type: paper
  citation: 'Ingolia, N. T., Ghaemmaghami, S., Newman, J. R., & Weissman, J. S. (2009). Genome-wide analysis in vivo of translation with nucleotide resolution using ribosome profiling. Science, 324(5924), 218–223.'
  url: 'https://doi.org/10.1126/science.1168978'
---
转录组测序（RNA-seq）让科学家对RNA全景了如指掌，但这掩盖了一个谎言：一条mRNA存在并不意味着它正在被翻译成蛋白质！细胞内充满着海量未翻译的非编码RNA或休眠转录本。到底哪些RNA真正骑着核糖体在轰鸣产出蛋白质？每个核糖体正停靠在哪个具体密码子上？2009年英戈利亚与魏斯曼发明了震撼分子生物学界的核糖体印迹测序（Ribo-seq）。其实验构思令人叹服：向快速生长的活细胞中迅速注入高浓度环己酰亚胺，该分子如同一剂强心麻醉针，几秒内卡死全细胞核糖体的移位通道，将所有核糖体冰冻在它们正在翻译的mRNA链条上！接着加入核酸酶RNase I水解：暴露在外的裸露单链RNA瞬间被全部啃食粉碎，唯独被核糖体严密包裹庇护的一截约30个核苷酸的mRNA小片段安然无恙（核糖体保护足迹, RPF）。将这些极短足迹分离建库深测序后，全细胞每一个核糖体的实时坐标以严格的3碱基周期性赫然显现，彻底打破了转录与翻译之间的黑盒盲区。
