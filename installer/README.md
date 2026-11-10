# HTTPS 游戏站安装说明

适用：Linux/systemd 主机，root/sudo，amd64 或 arm64，3X-UI 与游戏服务在同一网络命名空间。普通 Docker 容器内部不适用。

## 安装与检查

```bash
git clone https://github.com/gosinkx/h5game.git
cd h5game
sha256sum -c install.sh.sha256
sudo sh install.sh NODE_IP
```

`NODE_IP` 使用该节点证书匹配的 IPv4 地址，也支持匹配证书的 DNS 名。默认从证书唯一的 IPv4 SAN 自动识别，因此通常可省略。脚本要求已有有效面板证书，不申请或替换证书。

```bash
# 只检查，不安装或重启服务
sudo sh install.sh NODE_IP --check
# 正常重复运行保留游戏选择；此选项更换为另一款游戏
sudo sh install.sh NODE_IP --reroll
# 指定不同的证书路径
sudo sh install.sh NODE_IP --cert /path/fullchain.pem --key /path/privkey.pem
```

依赖：`systemctl openssl curl ss flock sha256sum tar gzip base64` 等系统工具。Debian/Ubuntu 缺少依赖时安装：

```bash
sudo apt-get update
sudo apt-get install -y ca-certificates curl openssl iproute2 util-linux coreutils tar gzip
```

## 安装结果

- `ecacc-cover.service` 开机自启，只监听 `127.0.0.1:8443`。
- 程序：`/opt/ecacc-cover/cover-site`；随机游戏选择：`/var/lib/ecacc-cover/state.json`。
- 校验证书身份、私钥配对及至少 24 小时有效期，拒绝覆盖其他程序占用的 8443。
- 验证程序/载荷摘要，并以正常 TLS 校验请求本机游戏站，检查正文摘要。
- 面板续期后，新 TLS 握手自动读取证书文件；不复制私钥进仓库或游戏目录。
- 更新前备份到 `/var/backups/ecacc-cover/<时间.随机后缀>/`；失败恢复原游戏文件、选择和服务状态。
- 不改防火墙，不抢占 TCP/UDP 443，不新建 Client，不更换原订阅、REALITY 密钥或 SNI，不添加 HY2。

新节点安装成功后仍需在 ECAcc 后台点击“备份并启用游戏回落”。仅安装服务并不会自动接管公网 443。后台可以“恢复原回落目标”，无需手动编辑 Xray。

已启用回落的节点重装会短暂重启游戏服务，应逐台执行并复核原客户端。直接访问游戏网页应使用证书匹配的节点 IP；第三方 SNI 不保证匹配节点 IP 证书。

## 更新和回退

```bash
git pull --ff-only
sha256sum -c install.sh.sha256
sudo sh install.sh NODE_IP
systemctl status ecacc-cover.service
```

正常更新保留所选游戏。恢复服务文件时先在 ECAcc 恢复原回落目标，再使用对应备份中的程序、state 和 unit 恢复，执行 `systemctl daemon-reload` 和服务重启；不要恢复旧面板库覆盖后来的用户修改。

## 维护者重新生成安装包

生成脚本需要开发机 Python 3 和 Go 1.22 以上（节点不需要）。

```bash
python installer/build.py
```

构建器读取本仓库的 `melon-lab/瓜体实验室.html`、`mosswing/mosswing.html`，更新内嵌资源摘要，先运行服务测试，再构建两种架构，生成根目录 `install.sh`、`install.sh.sha256`。游戏 HTML 变化后需重新构建并提交这两个文件，单纯修改 HTML 不会更新已发布的内嵌安装包。

构建中间文件位于被忽略的 `installer/.build/`，不会进入安装包。安装包只含游戏、服务二进制和安装源码，不含任何节点配置、证书、密钥或数据库。

游戏源及原始 Prompt 保留在原目录；HTTPS 安装器源于 ECAcc 的游戏回落部署工具。
