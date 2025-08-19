# 百度网盘文件上传工具 (Baidu Netdisk Uploader)

[![Go Version](https://img.shields.io/badge/Go-%3E%3D1.19-blue.svg)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/Platform-Windows%20%7C%20macOS%20%7C%20Linux-lightgrey.svg)](https://github.com)

一个基于百度网盘开放API的命令行文件上传工具，支持大文件分片上传、OAuth2.0授权、自动Token刷新等功能。

## ✨ 功能特性

- 🚀 **大文件支持**: 自动分片上传，支持任意大小文件
- 🔐 **OAuth2.0 授权**: 完整的授权流程，安全可靠
- 🔄 **自动Token刷新**: 智能检测和刷新过期的访问令牌
- 📊 **上传进度**: 实时显示上传进度和状态
- 🛠️ **断点续传**: 自动检测已存在文件，避免重复上传
- 💻 **跨平台**: 支持 Windows、macOS、Linux
- 🎯 **简单易用**: 命令行界面，操作简单直观

## 📋 目录

- [安装](#安装)
- [快速开始](#快速开始)
- [百度网盘应用配置](#百度网盘应用配置)
- [详细使用说明](#详细使用说明)
- [配置文件说明](#配置文件说明)
- [常见问题](#常见问题)
- [开发说明](#开发说明)
- [License](#license)

## 🚀 安装

### 方式一：下载预编译二进制文件
```bash
# 下载对应平台的二进制文件
# Windows: bddisk_uploader.exe
# macOS/Linux: bddisk_uploader
```

### 方式二：从源码编译
```bash
# 克隆项目
git clone <repository-url>
cd bddisk_uploader

# 编译
go build -o bddisk_uploader
```

## 🏃‍♂️ 快速开始

### 1. 初始化配置
```bash
./bddisk_uploader -init
```

### 2. 配置百度网盘应用信息
编辑生成的 `config.json` 文件，填入你的应用信息：
```json
{
  "oauth": {
    "client_id": "你的App Key",
    "client_secret": "你的Secret Key",
    "redirect_uri": "http://localhost:8080/callback"
  },
  "app_path": "/apps/你的应用名/"
}
```

### 3. 授权登录
```bash
./bddisk_uploader -auth
```

### 4. 上传文件
```bash
./bddisk_uploader -file /path/to/your/file.txt
```

## 🛠️ 百度网盘应用配置

### 创建应用

1. **访问百度网盘开放平台**
   
   打开 [https://pan.baidu.com/union/console](https://pan.baidu.com/union/console)

2. **登录和认证**
   - 使用百度账号登录
   - 完成实名认证（个人或企业认证）

3. **创建应用**
   - 点击"创建应用"按钮
   - 选择应用类型（推荐选择"软件"）
   - 填写应用信息：
     - **应用名称**: 自定义应用名称（0-20个字符）
     - **应用描述**: 应用功能描述
     - **授权回调地址**: `http://localhost:8080/callback`

4. **获取应用凭据**
   
   应用创建成功后，你将获得以下信息：
   - **App ID**: 应用ID（暂不用于OAuth流程）
   - **App Key**: 应用密钥，对应配置文件中的 `client_id`
   - **Secret Key**: 秘密密钥，对应配置文件中的 `client_secret`
   - **Sign Key**: 签名密钥（暂不使用）

### 配置说明

| 百度网盘控制台 | 配置文件字段 | 说明 |
|---------------|-------------|-----|
| App Key | `client_id` | OAuth2.0 客户端ID |
| Secret Key | `client_secret` | OAuth2.0 客户端密钥 |
| App ID | - | 不用于OAuth授权流程 |
| Sign Key | - | 用于其他API签名，OAuth不需要 |

### 重要提醒

- ⚠️ **Secret Key 要保密**: 不要将 Secret Key 提交到公开代码仓库
- ⚠️ **回调地址要匹配**: 确保在应用配置中设置正确的回调地址
- ⚠️ **权限范围**: 默认申请 `basic,netdisk` 权限，足够文件上传使用

## 📖 详细使用说明

### 命令行参数

#### 配置相关
```bash
-init                    # 初始化配置文件
```

#### 授权相关
```bash
-auth                    # 启动自动授权流程（推荐）
-code <授权码>           # 使用授权码手动获取access_token
-refresh-token           # 刷新过期的access_token
-port <端口>             # 指定授权回调服务器端口（默认8080）
```

#### 上传相关
```bash
-file <文件路径>         # 要上传的本地文件路径（必需）
-name <文件名>           # 上传到网盘的文件名（可选，默认使用本地文件名）
```

### 使用示例

#### 完整工作流程
```bash
# 1. 初始化配置
./bddisk_uploader -init

# 2. 编辑 config.json 填入应用信息

# 3. 自动授权
./bddisk_uploader -auth

# 4. 上传单个文件
./bddisk_uploader -file ~/Desktop/document.pdf

# 5. 上传并重命名
./bddisk_uploader -file ~/Downloads/video.mp4 -name "我的视频.mp4"
```

#### 授权相关示例
```bash
# 自动授权（会打开浏览器）
./bddisk_uploader -auth

# 使用自定义端口进行授权
./bddisk_uploader -auth -port 9090

# 手动输入授权码
./bddisk_uploader -code "4/0AY0e-g7X..."

# 刷新过期的token
./bddisk_uploader -refresh-token
```

#### 文件上传示例
```bash
# 上传图片
./bddisk_uploader -file ~/Pictures/photo.jpg

# 上传大文件（自动分片）
./bddisk_uploader -file ~/Downloads/large-file.zip

# 批量上传（使用脚本）
for file in ~/Documents/*.pdf; do
    ./bddisk_uploader -file "$file"
done
```

## ⚙️ 配置文件说明

### 配置文件结构
```json
{
  "access_token": "访问令牌",
  "refresh_token": "刷新令牌", 
  "expires_at": "2025-09-18T15:28:54.643061+08:00",
  "app_path": "/apps/你的应用名/",
  "oauth": {
    "client_id": "你的App Key",
    "client_secret": "你的Secret Key", 
    "redirect_uri": "http://localhost:8080/callback",
    "scope": "basic,netdisk"
  }
}
```

### 字段说明

| 字段 | 类型 | 必填 | 说明 |
|-----|------|------|-----|
| `access_token` | String | 否* | 访问令牌，授权后自动填充 |
| `refresh_token` | String | 否 | 刷新令牌，用于自动刷新access_token |
| `expires_at` | DateTime | 否 | access_token过期时间 |
| `app_path` | String | 是 | 文件上传路径前缀，必须以`/apps/应用名/`开头 |
| `oauth.client_id` | String | 是 | 百度网盘应用的App Key |
| `oauth.client_secret` | String | 是 | 百度网盘应用的Secret Key |
| `oauth.redirect_uri` | String | 否 | OAuth回调地址，默认为localhost:8080/callback |
| `oauth.scope` | String | 否 | 授权范围，默认为basic,netdisk |

*`access_token` 在首次使用时通过授权流程自动获取

### 配置文件管理

```bash
# 查看当前配置（隐藏敏感信息）
cat config.json | jq 'del(.access_token, .refresh_token, .oauth.client_secret)'

# 备份配置文件
cp config.json config.backup.json

# 重置配置（重新初始化）
rm config.json && ./bddisk_uploader -init
```

## 🔐 Token管理

### Token生命周期
- **Access Token**: 有效期 30 天
- **Refresh Token**: 有效期 10 年
- **自动刷新**: 程序会在access_token过期时自动使用refresh_token刷新

### 手动Token管理
```bash
# 检查token状态（查看过期时间）
cat config.json | jq '.expires_at'

# 手动刷新token
./bddisk_uploader -refresh-token

# 重新授权（如果refresh_token也过期）
./bddisk_uploader -auth
```

## ❓ 常见问题

### 应用配置相关

**Q: 如何获取App Key和Secret Key？**

A: 按以下步骤操作：
1. 访问[百度网盘开放平台](https://pan.baidu.com/union/console)
2. 登录百度账号并完成实名认证
3. 创建应用，选择"软件"类型
4. 在应用详情页面查看App Key和Secret Key

**Q: 授权时提示"unknown client id"？**

A: 检查以下几点：
- 确认App Key填写正确
- 确认应用状态为"正常"
- 确认在应用中正确设置了回调地址

**Q: 授权时提示"Client authentication failed"？**

A: 通常是Secret Key配置错误，请检查：
- 确认Secret Key填写正确
- 注意区分App Key和Secret Key
- 确保没有多余的空格或字符

### 使用相关

**Q: 端口8080被占用怎么办？**

A: 使用 `-port` 参数指定其他端口：
```bash
./bddisk_uploader -auth -port 9090
```

**Q: 授权时浏览器没有自动打开？**

A: 手动复制程序显示的授权URL到浏览器中打开

**Q: 上传大文件时速度很慢？**

A: 这是正常现象，大文件会自动分片上传：
- 文件按4MB分片上传
- 显示实时上传进度
- 支持断点续传

**Q: 上传的文件在网盘的哪里？**

A: 文件会上传到配置的`app_path`路径下，通常为`/apps/你的应用名/`

**Q: 支持上传文件夹吗？**

A: 当前版本仅支持单文件上传，可以使用脚本实现批量上传

### 错误处理

**Q: 提示"access_token已过期"？**

A: 程序会自动尝试刷新，如果失败请重新授权：
```bash
./bddisk_uploader -refresh-token
# 或者
./bddisk_uploader -auth
```

**Q: 上传失败怎么办？**

A: 检查以下几点：
- 网络连接是否正常
- access_token是否有效
- 文件路径是否正确
- 磁盘空间是否充足

## 🔧 开发说明

### 项目结构
```
bddisk_uploader/
├── main.go              # 主程序入口
├── auth.go              # OAuth2.0授权实现
├── go.mod               # Go模块文件
├── config.json          # 配置文件（运行时生成）
├── config.example.json  # 配置文件模板
├── README.md            # 项目说明文档
├── CHANGELOG.md         # 版本更新日志
├── .gitignore          # Git忽略规则
└── uploadsdk/          # 百度网盘SDK
    ├── upload/         # 文件上传API
    ├── utils/          # 工具函数
    └── demo/           # 示例代码
```

### 技术栈
- **语言**: Go 1.19+
- **HTTP客户端**: 标准库 net/http
- **JSON处理**: 标准库 encoding/json
- **CLI**: 标准库 flag
- **加密**: 标准库 crypto/md5

### 核心流程

#### 文件上传流程
1. **预创建** (`precreate`): 计算文件MD5，通知服务器准备接收
2. **分片上传** (`upload`): 按4MB分片上传文件内容  
3. **文件创建** (`create`): 合并所有分片，完成文件创建

#### OAuth授权流程
1. **获取授权码**: 用户在浏览器中授权，获得授权码
2. **获取Token**: 使用授权码换取access_token和refresh_token
3. **Token刷新**: 使用refresh_token刷新过期的access_token

### 构建和发布

```bash
# 本地构建
go build -o bddisk_uploader

# 交叉编译
# Windows
GOOS=windows GOARCH=amd64 go build -o bddisk_uploader.exe

# macOS  
GOOS=darwin GOARCH=amd64 go build -o bddisk_uploader-macos

# Linux
GOOS=linux GOARCH=amd64 go build -o bddisk_uploader-linux
```

### 贡献指南

1. Fork 本项目
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启 Pull Request

## 📝 版本历史

查看 [CHANGELOG.md](CHANGELOG.md) 了解版本更新详情。

## 🔒 安全提醒

- ❌ 不要将包含真实密钥的配置文件提交到公开代码仓库
- ✅ 使用环境变量或配置文件管理敏感信息
- ✅ 定期刷新access_token
- ✅ 及时更新应用密钥

## 🤝 支持

如果你觉得这个项目有帮助，请给个 ⭐️

## 📄 License

本项目采用 MIT 许可证 - 查看 [LICENSE](LICENSE) 文件了解详情。

---

**免责声明**: 本工具仅用于学习和个人使用，请遵守百度网盘的服务条款和相关法律法规。