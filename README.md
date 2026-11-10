# GPT-6 Astra · One Shot 游戏测试

## 3X-UI 节点一键安装 HTTPS 游戏回落站

在节点终端执行以下命令（将 IP 替换为当前节点 IP）：

```bash
git clone https://github.com/gosinkx/h5game.git
cd h5game
sudo sh install.sh 156.245.145.223
```

安装器内置本仓库两款游戏和 Linux amd64/arm64 服务程序，节点无需安装 Go、Python、Node.js，也不需要额外下载 GitHub Release。证书只有一个 IPv4 SAN 时，可以省略 IP：`sudo sh install.sh`。

默认直接读取 3X-UI 面板自动续期的证书：

- 公钥：`/root/cert/ip/fullchain.pem`
- 私钥：`/root/cert/ip/privkey.pem`

安装完成后，到 **ECAcc 后台 → 服务器 → 游戏回落 → 选择 REALITY 入站 → 备份并启用**。脚本只负责安装并验证本机 HTTPS 游戏服务，不直接修改 3X-UI 入站或 Client；由后台保存原目标，便于恢复。

详细选项、更新和备份说明见 [安装说明](installer/README.md)。下文保留原游戏介绍与 Prompt。

两次 One Shot，测试 Astra 的游戏设计与实现能力。保留原始 Prompt 和 HTML 产物，方便体验与 Fork。

## 01 · 瓜体实验室

半流体西瓜游戏，测试物理性质、形变与合成玩法。

[在线体验](https://melon-game.jack-514.chatgpt.site) · [HTML](melon-lab/瓜体实验室.html) · [源码](melon-lab/src)

```text
用最短的时间设计一款全新概念（物理性质）的半流体西瓜游戏（html游戏）
```

## 02 · Mosswing

上一轮 Prompt 加了时间限定词，这次去掉时间限定，测试移动端 3D 单键飞行游戏。同样 One Shot，**Xhigh，用时 21 分钟**。

[在线体验](https://mosswing-quiet-flight.jack-514.chatgpt.site/) · [HTML](mosswing/mosswing.html) · [源码](mosswing/src)

```text
Remaster the classic "tap-to-flap" game — the one where you tap to keep a small creature airborne while gliding through an endless series of gaps — as a 3D game playable in a mobile browser. One index.html, opens and plays instantly, no external assets (CDN libraries are allowed; your call).  Keep the core exactly as everyone remembers it: one-tap control, gravity, gaps that scroll toward you, one hit and you're done, score is gaps passed. Everything else is yours to decide: what the creature is, what the obstacles are, the world, the camera, the feel of the flap, how far to take the visuals. Design an original character and style rather than copying the original's art. I won't answer clarifying questions.  I'm judging a complete, elegant, great-feeling piece of work — not a feature list. Small and finished beats big and rough.
```

## 运行

下载对应 HTML，用现代浏览器直接打开即可，无需安装依赖或构建。
