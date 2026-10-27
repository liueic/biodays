---
id: BIO-C2-24
title: 不在其位却下其剪的魔术师：Golden Gate 金门组装与 Type IIS 限制酶
lab_item: Type IIS 限制性内切酶（BsaI / BsmBI）、T4 DNA 连接酶与 PCR 热循环仪
year: 2008
month: 11
day: 10
tags:
  - 金门组装
  - Type IIS酶
  - 合成生物学
summary: 传统回文限制酶切点必定残留在接头破坏读框；Type IIS酶在识别序列外精准下剪，实现一管式多片段无缝且不可逆的高效组装。
geek_quote: 最完美的缝合，就是在剪断边缘的同时，将剪刀的名字从图纸上彻底抹去。
source:
  type: paper
  citation: 'Engler, C., Kandzia, R., & Marillonnet, S. (2008). A one pot, one step, modular cloning method for assembly of DNA fragments into a vector. PLoS ONE, 3(11), e3647.'
  url: 'https://doi.org/10.1371/journal.pone.0003647'
---
传统限制性内切酶（如EcoRI、HindIII）的致命软肋在于：它们必须在自己的回文识别序列内部切割，这意味着拼接完成后，接头处必定会永远烙下一个6个碱基的人工切位点，不仅破坏蛋白质密码子阅读框，还会导致同一个酶无法再次使用。2008年马里隆内特团队将目光投向了长期被视作“边角料”的Type IIS型限制酶（如BsaI、BsmBI）。这种酶拥有极其不可思议的空间催化构型：它的DNA结合域紧紧识别一段非对称序列（如GGTCTC），但其剪切催化结构域却像一根长长的悬臂吊车，伸到识别序列外侧1到5个碱基处“隔空下剪”！这产生了一个革命性的热力学不可逆单向通道：实验人员在设计重组片段时，让两端的BsaI识别序列朝向内侧，切割后释放出的4个碱基黏性末端由人工自由定制。当把数十个DNA模块、BsaI酶和T4连接酶统统扔在同一个反应管中进行37°C酶切与16°C连接的交替热循环时，切下的片段彼此拼接后，原先的BsaI识别序列被彻底永久剔除！正确拼接物因无法被再次切割而无限单向积累，一次性无缝拼装上百个零件，一跃成为合成生物学与TALEN构建的终极装配线。
