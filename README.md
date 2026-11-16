# mygo-pv — 45 秒 3:4 的 MyGo render 介绍片

一条 45 秒、1080×1440、30 fps 的动效片，为一个 Go 的 UI 工具包（`github.com/egoist/mygo` 的 `render` 插件）做介绍。

**整条片子没有一个素材文件、没有浏览器、没有一帧外部动画**：画面是 `ui` 包的原生视图（文本、盒子、真正的 Progress / Switch / Slider 控件），由 GPU 直接画进编码器的 IOSurface 再压成 H.264；声音是 Go 里现算的 48 kHz 立体声。片子里那句 `$ go run ./pv` 说的就是它自己。

https://github.com/user-attachments/assets/e9fb9c46-7fee-4a7f-a6fa-c5a5b6bc1ae7

完整 45 秒成片（含音轨）可直接播放；原始文件在 [`renders/mygo-pv.mp4`](renders/mygo-pv.mp4)。

---

## 跑起来

```sh
cd mygo-pv

go run ./pv                                  # 渲整条 → renders/mygo-pv.mp4
go run ./pv -o /tmp/pv.mp4                   # 换个输出
go run ./pv -stills 130,450,1080             # 只出这几帧的 PNG，用来迭代
go run ./pv -progress=false                  # 不要进度条
```

只需要 Go（`go.mod` 写的是 1.27.4）。`vendor/` 已经带上了，**不需要联网**就能编译。

一条 45 秒的片子在本机（Apple M2 Pro / darwin-arm64 / Metal）上约 **31 秒**渲完。

---

## 文件

```
pv/main.go          composition：七个段落、时间线、所有元素
sound/score.go      纯 Go 合成器：和弦、冲击、拨弦、噪声、混音与限制器
assets/             Geist / GeistMono / Anton 三套字体（OFL）
renders/            成片、海报帧、45 帧总览
vendor/             mygo 及依赖的源码，离线编译用
```

---

## 七个段落

| 帧 | 时间 | 内容 |
|---|---|---|
| 0000–0071 | 0.0–2.4 s | 黑场里一个光点长成一片过曝的白 —— 不是淡入，是打亮 |
| 0072–0239 | 2.4–8.0 s | 白场上 `MyGo!!!!!` 的五个 `!` 逐拍砸落，`render` 推上来，命令行逐字敲入 |
| 0240–0539 | 8.0–18.0 s | **五刀**，一刀一个叹号：NATIVE UI / GPU DRAWN / 60 A SECOND / MOTION BLUR / SEVEN MEGS |
| 0540–0779 | 18.0–26.0 s | `THE FRAME IS THE APP`：三维偏转的窗口里，真 Progress、Switch、Slider 在动 |
| 0780–0989 | 26.0–33.0 s | `1350 FRAMES`：滚动的片格带 + 播放头 |
| 0990–1139 | 33.0–38.0 s | `IT HAS A SOUNDTRACK`：**把自己音轨的采样画出来**，一边画一边放 |
| 1140–1349 | 38.0–45.0 s | 白场落版，`mygo.run`，`$ go run ./pv` |

---

## 三处自指

1. **命令自指** —— 开场敲进去的就是渲出这条片子的命令。
2. **画面自指** —— 第 5 段那个窗口里的 Progress / Switch / Slider 是真的控件，被同一个渲染调用拍成了视频，不是画的假 UI。
3. **音频自指** —— 第 6 段画的是这条 45 秒音轨**自己的包络**。`Envelope()` 把混音切成 168 个峰值，播放头走过去一格亮一格。

---

## 五个叹号

不是随便排的。作者站点（mygo.run）的 `logo.tsx` 里，五个 `!` 各自旋转：

```
-rotate-9   rotate-6   -rotate-4   rotate-8   -rotate-6
```

"各自歪"才是这五个人的意思，整齐划一反而错。落版时旋转从 0 展开到各自的角度。

配色是从参考 PV 的落版上一个一个采样出来的（`#206488 → #5B3B73`，冷蓝走到暗紫），提亮成 `#2E86B4 → #7E46AC`，好让同一组颜色在白底黑底都立得住 —— 所以标题、五个切口、落版共用一套色。

---

## 声音

120 BPM、22.5 小节。合成器只有纯 Go：正弦、噪声、单极点滤波器、一段延迟线。

- **拨弦**是 Karplus-Strong（噪声激励延迟线 + 平均低通自我反馈），所以终于有 riff 了。
- **冲击**（`Hit`）是复合的：40→92 Hz 下潜 + 听得见音高的 body + 1.9 kHz 瞬态 + 2.6 kHz 的 crack，全长 0.85 秒。**每次单独出现，中间留静音** —— 这是参考 PV 开场的全部语法。
- 五个 `!` 落地在 2.4 / 2.9 / 3.4 / 3.9 / 4.4 s，是 A 小调五声音阶的五个拨弦音，每个配一记冲击。
- 段落之间按段落给量：8–18 s 全鼓 → 18–26 s 鼓全撤 → 26–33 s 加十六分 hats 往上推 → 33–38 s 再撤 → 38 s 落版。

### 频谱（实测，取整）

| 频段 | 本片 | 参考 PV |
|---|---|---|
| 0–120 Hz | 12.8% | 3.6% |
| 120–400 Hz | 50.2% | 27.5% |
| 400–1200 Hz | 33.2% | 39.5% |
| 1200–3000 Hz | 1.4% | 15.9% |
| 3000–8000 Hz | 1.1% | 8.8% |

第一版 70% 的能量压在 120 Hz 以下、400–1200 Hz 只有 5.7%，听感是一团三十秒的低频轰鸣。重写后中频回到 33%。**高频仍然比参考薄** —— 参考是成品混音，这里是全合成器，硬推只会变刺，这是能力边界不是没试。

### 输出链

末尾挂的是**带 3 ms 前瞻、70 ms 释放、双声道联动的限制器**，不是 `tanh` 波整形。区别很实在：

| | tanh | 前瞻限制器 |
|---|---|---|
| 真峰值 | −0.1 dBFS | **−1.0 dBFS** |
| 响度 | −17.8 LUFS | **−15.8 LUFS** |

峰值降了 0.9 dB，响度反而涨 2 dB。

---

## 实测

| | |
|---|---|
| 1350 帧，5 采样运动模糊（Shutter 320） | **30.8 s**（43.8 fps） |
| 产物 | H.264 High / yuv420p / 1080×1440 / 1350 帧 / 45.000000 s / **8.59 MB** / AAC-LC 48 kHz 立体声 |
| 响度 | **−15.8 LUFS** / −1.0 dBFS 真峰值 / LRA 4.2 LU |

参考的那条官方 PV：1920×1080 / 23.976 fps / 54.1 s / 5.27 MB / −16.2 LUFS / +0.2 dBFS 真峰值 / LRA 17.8。

---

## 用到的 API

这篇的价值一半在于它把 `render` 插件能用到的部分基本都用了一遍：

- `render.Composition{Width, Height, FPS, Frames, Scale, Dark, Background, MotionBlur}`
- `Frame`：`Index` / `Count` / `Exact()` / `Progress()` / `Audio()` / `Sequence()` / `Shift()` / `Interpolate()`
- **运动模糊**：`MotionBlur{Samples: 5, Shutter: 320}` —— 凡是靠 `Exact()` 驱动的运动都会糊，靠 `Index` 的不会
- `render.Layer()` 叠层、`render.Still()` 单帧、`render.Video()` 出片
- `ui`：`RadialGradient`（带 `Oklab` 插值）、`LinearGradient`、`Skew`、`RotateX/Y` + `Perspective`、`TextStroke`、`Border`、`Blur`、`Shadow`、`Scale`、`Clip`、`Themed` + `DarkTheme`
- 真控件：`ui.Progress` / `ui.Switch` / `ui.Slider`
- `ui.RegisterFont` + 可变字体字重

---

## 踩过的坑

1. **两个 `Fill()` 兄弟会把画面劈成两半**。`ui.Box(c).Fill()` 在 Column 里是和兄弟平分高度的；想问"铺满"必须各自套一层 `render.Layer`。
2. **但 `render.Layer` 当兄弟又会吃掉布局空间**：背景 Layer 和内容 Column 并列后，内容整段消失。最后把全屏底色收进一个 `backdrop()`，每帧只允许一处全屏涂抹。
3. **`render.Layer` 里的绝对定位是相对那个 Layer 的**，不是相对整幅。`Top()` 按整幅写就会飞到画面外。
4. **`Scale()` 会连边距一起缩放**。想在静止画面上"缓慢推近"，用 `Translate`，否则边距会漂。
5. **`Sequence` 里的 `Frame` 已经是局部的**，`f.Index` 从段落 0 起，不需要再 `Shift`。
6. **`Gradient(from, to, angle)` 的 90° 是左→右**，180° 才是上→下（和 CSS 一致）。
7. **`ui.Progress` 要 `float64`，`ui.Slider` 要 `*float64`** —— 不是 `float32`。
8. 文本排版会吃掉行尾空格，代码片段里的空格要用 NBSP（`\u00a0`）。

---

## 素材与许可

- 字体：Geist / Geist Mono（Vercel，OFL）、Anton（OFL），随包附带。
- `MyGo!!!!!` 的字形写法参考了 mygo.run 站点与其官方视觉，仅作技术演示。
- 画面与声音全部由本仓库的 Go 代码生成，不含任何第三方素材。
