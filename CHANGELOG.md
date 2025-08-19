# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2025-08-19

### Added
- ✨ **核心功能**
  - 文件上传到百度网盘的完整功能
  - 大文件自动分片上传（4MB分片）
  - 文件MD5校验和完整性验证
  - 断点续传支持（自动跳过已存在文件）

- 🔐 **OAuth2.0 授权系统**
  - 完整的OAuth2.0授权码流程实现
  - 自动启动本地HTTP服务器接收授权回调
  - 手动授权码输入支持
  - Access Token和Refresh Token管理
  - 自动Token过期检测和刷新

- 🛠️ **用户体验**
  - 命令行界面，操作简单直观
  - 实时上传进度显示
  - 详细的错误提示和处理
  - 配置文件管理系统
  - 跨平台支持（Windows、macOS、Linux）

- 📝 **文档和配置**
  - 详细的README文档
  - 完整的百度网盘应用配置指南
  - 配置文件模板和示例
  - 常见问题解答
  - 项目结构说明

### Technical Details
- **Language**: Go 1.19+
- **Dependencies**: 仅使用Go标准库
- **Architecture**: 
  - `main.go`: 主程序入口和文件上传逻辑
  - `auth.go`: OAuth2.0授权实现
  - `uploadsdk/`: 百度网盘API SDK

### Command Line Interface
```bash
# 配置管理
-init                    # 初始化配置文件

# 授权相关
-auth                    # 自动授权流程
-code <code>             # 手动授权码输入
-refresh-token           # 刷新访问令牌
-port <port>             # 自定义回调端口

# 文件上传
-file <path>             # 上传文件
-name <name>             # 自定义远程文件名
```

### Security Features
- 配置文件中的敏感信息保护
- OAuth2.0标准安全流程
- Token自动过期管理
- 本地临时文件自动清理

### Supported File Operations
- ✅ 单文件上传
- ✅ 大文件分片上传
- ✅ 文件重命名
- ✅ 重复文件检测
- ✅ 上传进度显示

### Known Limitations
- 暂不支持文件夹批量上传
- 暂不支持断点续传（已上传分片的恢复）
- 需要有效的百度网盘开放平台应用

---

## Development Notes

### Initial Release Highlights
这是百度网盘文件上传工具的首个正式版本。该工具从零开始开发，包含了完整的文件上传和OAuth授权功能。

### Key Implementation Details
1. **文件上传采用百度网盘标准三步流程**:
   - Precreate: 预创建文件，获取上传ID
   - Upload: 分片上传文件内容
   - Create: 合并分片，完成文件创建

2. **OAuth2.0授权流程严格遵循RFC标准**:
   - Authorization Code Grant流程
   - PKCE支持（通过state参数）
   - 安全的Token存储和管理

3. **错误处理和用户体验优化**:
   - 详细的错误信息和解决建议
   - 自动重试机制
   - 优雅的程序退出和资源清理

### Future Roadmap
- [ ] 支持文件夹批量上传
- [ ] 支持上传进度暂停/恢复
- [ ] 支持多线程并行上传
- [ ] 增加配置文件加密选项
- [ ] 支持更多百度网盘API功能（下载、删除等）