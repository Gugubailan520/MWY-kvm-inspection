# 违规巡查系统（KVM/LXC 网络监控）

基于 Go 的 KVM/LXC 网络违规巡查系统，包含 Agent（客户端）、Server（服务端）、Web（前端）三部分。

详细设计见 [开发文档.md](开发文档.md)。

## 目录结构

```text
.
├── Agent/      # 客户端：宿主机抓包 + DPI 识别 + iptables 封禁
├── Server/     # 服务端：API + WebSocket + MySQL + MongoDB
├── Web/        # 前端：Vue3 + Arco Design
├── config/     # 配置模板
├── deploy/     # 部署脚本 & systemd 服务文件
└── 开发文档.md
```

## 快速开始

### 启动服务端

```bash
cd Server
cp ../config/server.yaml.example ../config/server.local.yaml
# 编辑 server.local.yaml 填写数据库连接
go run ./cmd/server -c ../config/server.local.yaml
```

### 启动 Agent

```bash
cd Agent
cp ../config/agent.yaml.example ../config/agent.local.yaml
# 编辑 agent.local.yaml 填写 server 地址与 api_key
go run ./cmd/agent -c ../config/agent.local.yaml
```

### 启动前端

```bash
cd Web
npm install
npm run dev
```

## 构建发布

> **注意**：Agent 依赖 `libpcap`（cgo），必须在 Linux 构建主机上安装开发库后编译：
> - CentOS/RHEL: `yum install -y libpcap-devel gcc`
> - Debian/Ubuntu: `apt-get install -y libpcap-dev gcc`

```bash
# 在 Linux 构建主机执行（自动编译 server 与 agent 到 bin/）
bash deploy/build.sh
```
