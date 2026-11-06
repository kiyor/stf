# STF (简单传输工具)

[![许可证: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go 版本](https://img.shields.io/badge/Go-%3E%3D1.19-blue)](https://golang.org/)
[![Docker](https://img.shields.io/badge/Docker-Available-blue)](https://hub.docker.com/r/kiyor/stf)

STF 是一个用 Go 语言编写的多功能网络工具，在单个二进制文件中提供 HTTP 文件服务器、SOCKS5 代理、TCP 代理、HTTP 隧道和测试文件生成功能。

## 🚀 特性

- **HTTP 文件服务器** - 提供文件服务，支持上传和目录列表
- **SOCKS5 代理** - 功能完整的 SOCKS5 代理，支持身份认证和速率限制
- **TCP 代理** - 原始 TCP 连接代理
- **HTTP 隧道** - HTTP CONNECT 隧道代理
- **测试文件生成器** - 动态生成测试文件用于带宽测试
- **桥接模式** - HTTP/HTTPS 转发，自动协议检测
- **身份认证** - HTTP 基本认证和 SOCKS5 认证
- **速率限制** - SOCKS5 可配置带宽限制
- **安全特性** - IP 允许/拒绝列表，超时控制

## 📦 安装

### 预编译二进制文件
从 [releases](https://github.com/kiyor/stf/releases) 下载或使用 Docker：

```bash
docker run --rm -p 30000:30000 kiyor/stf
```

### 源码编译
```bash
git clone https://github.com/kiyor/stf.git
cd stf
go build -v
```

## 🎯 快速开始

### 基本 HTTP 文件服务器
```bash
# 在端口 30000 上服务当前目录
./stf

# 指定目录和自定义端口
./stf -d /path/to/files -p :8080

# 启用文件上传
./stf -uploadonly
```

在浏览器访问 `http://localhost:30000`

### SOCKS5 代理
```bash
# 基本 SOCKS5 代理
./stf -socks5 -p :1080

# 带身份认证
echo "user:password" > auth.txt
./stf -socks5 -socks5auth auth.txt -p :1080

# 配置应用程序使用 SOCKS5 代理 localhost:1080
```

### HTTP 隧道代理
```bash
# 启动 HTTP 隧道
./stf -tunnel -p :8888

# 配置应用程序使用 HTTP 代理 localhost:8888
```

## 📖 详细用法

### HTTP 文件服务器模式

默认模式从目录提供文件服务，支持可选的上传功能。

#### 基本文件服务
```bash
# 服务当前目录
./stf

# 服务指定目录
./stf -d /var/www/html

# 自定义端口
./stf -p :8080

# 禁用 gzip 压缩
./stf -gzip-disable
```

#### 文件上传
```bash
# 启用仅上传模式（只接受 POST/PUT 请求）
./stf -uploadonly

# 使用 curl 上传文件
curl -X POST -F "file=@example.txt" http://localhost:30000/upload
```

#### 身份认证
```bash
# 简单的用户名:密码格式
./stf -httpauth "user:secret123"

# 从文件读取（每行一个 用户名:密码）
echo -e "admin:password\nuser:123456" > auth.txt
./stf -httpauth auth.txt

# JSON 格式
echo '{"users": {"admin": "secret", "user": "pass"}}' > auth.json
./stf -httpauth auth.json
```

#### 安全和访问控制
```bash
# 允许特定 IP 范围
./stf -allow 192.168.1.0/24 -allow 10.0.0.1

# 拒绝特定 IP
./stf -deny 1.2.3.4 -deny 5.6.7.8

# 自定义超时时间（默认 5 分钟）
./stf -timeout 10m

# 禁用超时
./stf -notimeout
```

#### 高级选项
```bash
# 详细日志记录
./stf -v          # 基本详细模式
./stf -vv         # 非常详细（包含请求/响应体）
./stf -body       # 显示请求/响应体内容
./stf -c          # 彩色输出

# 调试响应
./stf -return 404          # 始终返回 404
./stf -3xx "301 https://example.com"  # 重定向所有请求

# 自定义 gzip 类型
./stf -gzip-types "text/html application/json"
```

### SOCKS5 代理模式

STF 提供功能完整的 SOCKS5 代理服务器，支持身份认证和速率限制。

#### 基本 SOCKS5 代理
```bash
# 在端口 1080 启动 SOCKS5 代理
./stf -socks5 -p :1080

# 自定义端口
./stf -socks5 -p :9050
```

#### 身份认证
```bash
# 基于文件的认证（每行 用户名:密码）
echo -e "user1:pass1\nuser2:pass2" > socks_auth.txt
./stf -socks5 -socks5auth socks_auth.txt

# JSON 格式
echo '{"users": {"proxy_user": "secret123"}}' > socks_auth.json
./stf -socks5 -socks5auth socks_auth.json
```

#### 速率限制
```bash
# 限制上传到 1MB/s，下载到 5MB/s
./stf -socks5 -socks5in 1048576 -socks5out 5242880

# 无上传限制，10MB/s 下载限制
./stf -socks5 -socks5out 10485760
```

#### 代理链
```bash
# 通过另一个 SOCKS5 代理链接
./stf -socks5 -socks5next upstream.proxy.com:1080

# 带认证的链接
./stf -socks5 -socks5next proxy.com:1080:user:pass

# 禁用 DNS 解析（让上游处理）
./stf -socks5 -socks5next proxy.com:1080 -socks5noresolver
```

#### 主机文件覆盖
```bash
# 自定义主机解析
echo -e "example.com 127.0.0.1\ntest.local 192.168.1.100" > hosts.txt
./stf -socks5 -socks5hosts hosts.txt
```

### TCP 代理模式

将原始 TCP 连接转发到上游服务器。

```bash
# 将 TCP 连接代理到后端服务器
./stf -upstream backend.example.com:3306 -p :3306

# 带 IP 限制的代理
./stf -upstream 10.0.0.100:5432 -allow 192.168.1.0/24 -p :5432
```

### HTTP 隧道模式

为 HTTPS 和其他协议创建 HTTP CONNECT 隧道代理。

```bash
# 启动 HTTP 隧道代理
./stf -tunnel -p :8888

# 带身份认证
./stf -tunnel -httpauth "proxy_user:secret" -p :8888

# 配置应用程序使用 HTTP 代理 localhost:8888
```

### 桥接模式

自动将 HTTP 和 HTTPS 流量转发到不同的上游服务器。

```bash
# 转发到单个后端（自动检测 HTTP/HTTPS）
./stf -bridge backend.internal.com

# 将 HTTP 转发到一个服务器，HTTPS 转发到另一个
./stf -bridge "http.internal.com:https.internal.com"

# 自定义端口
./stf -bridge "internal.com:192.168.1.100" -p :80

# 带 SSL 证书的 HTTPS 终端
./stf -bridge internal.com -crt server.crt -key server.key
```

### 测试文件生成器

生成指定大小的文件用于带宽和性能测试。

```bash
# 启用测试文件生成
./stf -testfile -p :8080

# 访问测试文件：
# http://localhost:8080/1K    - 1 千字节文件
# http://localhost:8080/10M   - 10 兆字节文件
# http://localhost:8080/1G    - 1 千兆字节文件

# 自定义缓存控制
./stf -testfile -testcc "no-cache"

# 启用带宽计数器
./stf -testfile -testc

# 支持 HTTP Range 的部分下载
curl -H "Range: bytes=0-1023" http://localhost:8080/10M
```

#### 测试文件 URL
- `/1K` 到 `/999K` - 千字节文件
- `/1M` 到 `/999M` - 兆字节文件  
- `/1G` 到 `/999G` - 千兆字节文件
- `/1T` 到 `/999T` - 太字节文件

## 🔧 配置示例

### 完整 Web 服务器设置
```bash
# 带认证和安全控制的生产 Web 服务器
./stf -d /var/www \
      -p :80 \
      -httpauth auth.txt \
      -allow 0.0.0.0/0 \
      -deny 192.168.100.0/24 \
      -timeout 30m \
      -v
```

### 企业代理设置
```bash
# 带速率限制和认证的 SOCKS5 代理
./stf -socks5 \
      -p :1080 \
      -socks5auth corporate_users.txt \
      -socks5in 5242880 \
      -socks5out 10485760 \
      -allow 192.168.0.0/16 \
      -v
```

### 开发测试服务器
```bash
# 带上传和测试文件的测试服务器
./stf -d ./public \
      -uploadonly \
      -testfile \
      -testc \
      -p :3000 \
      -vv \
      -c
```

### 负载均衡桥接
```bash
# 用于负载均衡的桥接模式
./stf -bridge "app1.internal:app2.internal" \
      -p :80 \
      -allow 10.0.0.0/8 \
      -timeout 60s \
      -v
```

## 🐳 Docker 使用

### 基本文件服务器
```bash
# 服务当前目录
docker run --rm -p 30000:30000 -v $(pwd):/data kiyor/stf -d /data

# 自定义配置
docker run --rm -p 8080:8080 -v $(pwd):/data kiyor/stf -d /data -p :8080 -uploadonly
```

### SOCKS5 代理
```bash
# 带认证的 SOCKS5 代理
echo "user:pass" > auth.txt
docker run --rm -p 1080:1080 -v $(pwd)/auth.txt:/auth.txt kiyor/stf -socks5 -socks5auth /auth.txt -p :1080
```

### 使用 Docker Compose 持久化数据
```yaml
version: '3.8'
services:
  stf:
    image: kiyor/stf
    ports:
      - "30000:30000"
    volumes:
      - ./files:/data
      - ./auth.txt:/auth.txt
    command: [-d, /data, -httpauth, /auth.txt, -uploadonly, -v]
    restart: unless-stopped
```

## 📊 性能和监控

### 详细日志
```bash
# 基本请求日志
./stf -v

# 详细请求/响应日志
./stf -vv

# 记录到文件并带时间戳
./stf -v 2>&1 | tee stf.log
```

### 调试模式
```bash
# 将请求转储到 /tmp/stfdump/（使用 -v 或 -vv）
./stf -vv  # 在 /tmp/stfdump/ 创建详细转储

# 监控连接和带宽
./stf -socks5 -v  # 显示连接统计
```

## 🔐 安全最佳实践

### 身份认证
- 在认证文件中使用强密码
- 定期轮换凭据
- 对于复杂认证设置优选 JSON 格式

### 网络安全
```bash
# 限制访问到可信网络
./stf -allow 192.168.1.0/24 -allow 10.0.0.0/8 -deny 0.0.0.0/0

# 使用合理的超时时间
./stf -timeout 5m
```

### 文件服务器安全
```bash
# 只服务特定目录
./stf -d /var/www/public

# 为敏感内容启用认证
./stf -httpauth "admin:$(openssl rand -base64 32)"
```

## 🚨 故障排除

### 常见问题

**端口已被占用：**
```bash
# 检查什么在使用端口
sudo netstat -tlnp | grep :30000

# 使用不同端口
./stf -p :30001
```

**权限被拒绝：**
```bash
# 对于端口 < 1024，以 root 运行或使用更高端口
sudo ./stf -p :80
# 或者
./stf -p :8080
```

**连接超时：**
```bash
# 增加超时时间或禁用它
./stf -timeout 10m
# 或者
./stf -notimeout
```

**文件上传问题：**
```bash
# 确保启用上传模式且目录可写
./stf -uploadonly -d /tmp/uploads
chmod 755 /tmp/uploads
```

### 调试模式
```bash
# 启用最大详细程度
./stf -vv -body -c

# 检查配置
./stf -version
```

### 日志分析
```bash
# 监控实时日志
./stf -v | grep -E "(ERROR|WARN|INFO)"

# 记录到文件并轮转
./stf -v 2>&1 | rotatelogs stf_%Y%m%d.log 86400
```

## 🎁 使用场景

### Web 开发
```bash
# 本地开发服务器
./stf -d ./dist -p :3000 -uploadonly -v -c

# 静态文件 CDN
./stf -d /var/www/static -p :80 -timeout 30m
```

### 网络测试
```bash
# 带宽测试服务器
./stf -testfile -testc -p :8080

# 测试下载速度
curl -w "%{speed_download}\n" -o /dev/null http://server:8080/100M
```

### 代理服务
```bash
# 企业内网代理
./stf -socks5 -socks5auth users.txt -allow 192.168.0.0/16 -p :1080

# HTTP 隧道代理（翻墙）
./stf -tunnel -httpauth proxy_users.txt -p :8888
```

### 文件共享
```bash
# 临时文件共享
./stf -d /tmp/share -uploadonly -timeout 1h -p :8000

# 安全文件服务器
./stf -d /secure/files -httpauth admin:$(date +%s | sha256sum | head -c 16) -allow 192.168.1.0/24
```

### 系统管理
```bash
# 服务器间文件传输
# 服务器 A
./stf -d /data/backup -p :30000

# 服务器 B
wget -r http://server-a:30000/

# 负载均衡测试
./stf -bridge "backend1.local:backend2.local" -v -p :80
```

## 🤝 贡献

1. Fork 仓库
2. 创建特性分支：`git checkout -b feature/amazing-feature`
3. 提交更改：`git commit -m '添加令人惊叹的功能'`
4. 推送到分支：`git push origin feature/amazing-feature`
5. 开启 Pull Request

## 📝 许可证

本项目基于 MIT 许可证 - 查看 [LICENSE](LICENSE) 文件了解详情。

## 🙏 致谢

- 使用 Go 和标准库构建
- 使用 `github.com/NYTimes/gziphandler` 进行压缩
- 基于 `github.com/kiyor/go-socks5` 的 SOCKS5 实现
- 通过 `github.com/juju/ratelimit` 进行速率限制

## 📞 支持

- 🐛 **错误报告：** [GitHub Issues](https://github.com/kiyor/stf/issues)
- 💡 **功能请求：** [GitHub Discussions](https://github.com/kiyor/stf/discussions)
- 📖 **文档：** [Wiki](https://github.com/kiyor/stf/wiki)

---

**STF** - 简单、强大、多功能。一个二进制文件，多个协议。 🚀