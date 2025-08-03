# 🚀 GitHub推送工具套件使用指南

本工具套件包含三个脚本，帮助您轻松管理GitHub项目推送。

## 📦 工具套件组成

### 1. `setup-github-ssh.sh` - SSH密钥设置助手
**用途**：首次使用时配置GitHub SSH访问
**功能**：
- 自动生成SSH密钥对
- 显示公钥内容
- 指导添加到GitHub
- 测试SSH连接

### 2. `push-to-github.sh` - 完整推送脚本
**用途**：新项目推送或完整项目管理
**功能**：
- 自动检测项目名称
- 智能仓库管理
- 新建/更新模式
- 自动配置Git
- 创建.gitignore文件
- **🆕 清除其他GitHub账户配置**
- **🆕 默认时间戳提交信息**

### 3. `quick-push.sh` - 快速推送脚本
**用途**：已配置项目的快速更新
**功能**：
- 快速提交和推送
- 自动处理合并冲突
- 支持自定义提交信息
- **🆕 自动检查GitHub账户配置**
- **🆕 默认时间戳提交信息**

### 4. `clean-github-config.sh` - 配置清理工具
**用途**：彻底清除所有GitHub相关配置
**功能**：
- 清除全局Git配置
- 清除本地仓库远程配置
- 清理SSH known_hosts
- 清除Git凭据存储
- 设置为目标账户配置

## 🎯 使用流程

### 第一次使用（新用户）

#### 步骤0：清理现有配置（可选）
如果之前使用过其他GitHub账户，建议先清理：
```bash
./clean-github-config.sh
```

#### 步骤1：配置SSH密钥
```bash
./setup-github-ssh.sh
```

**脚本会自动：**
1. 生成SSH密钥对
2. 显示公钥内容
3. 指导您添加到GitHub
4. 测试连接

**手动操作：**
1. 复制显示的SSH公钥
2. 访问 https://github.com/settings/keys
3. 点击"New SSH key"
4. 粘贴公钥并保存

#### 步骤2：推送项目
```bash
./push-to-github.sh
```

### 日常使用（已配置用户）

#### 快速推送更新
```bash
./quick-push.sh "修复登录bug"
```

#### 或交互式推送
```bash
./quick-push.sh
# 然后输入提交信息
```

#### 新项目推送
```bash
./push-to-github.sh
```

## 🆕 新功能特性

### 🧹 自动清除其他GitHub配置
- **智能检测**：自动检测是否连接到其他GitHub账户
- **自动清理**：清除所有非目标账户的远程仓库配置
- **安全保障**：确保只连接到您的GitHub账户 (proregmao)

### ⏰ 智能默认提交信息
- **更新提交**：不输入信息时自动使用时间戳 `Update 2025-01-15 14:30:25`
- **首次提交**：自动使用 `Initial commit for 项目名`
- **交互友好**：显示默认信息，可选择使用或自定义

### 🔧 配置清理工具
新增 `clean-github-config.sh` 脚本：
```bash
./clean-github-config.sh
```
**功能**：
- 清除全局Git配置中的其他账户信息
- 清除本地仓库的所有远程配置
- 清理SSH known_hosts中的GitHub条目
- 清除Git凭据存储
- 重新设置为目标账户配置

### 🛡️ 智能错误处理
**新增强大的错误处理机制**：

#### **仓库不存在错误**
```
[ERROR] 推送失败！
错误信息: Repository not found

[WARNING] GitHub仓库不存在，请选择解决方案：
1) 创建新的GitHub仓库
2) 使用不同的项目名称
3) 手动创建仓库后重试
4) 退出脚本
请选择 [1-4]:
```

#### **权限错误处理**
```
[WARNING] 权限被拒绝，可能的解决方案：
1) 检查SSH密钥配置
2) 重新设置SSH密钥
3) 退出脚本
```

#### **网络错误处理**
```
[WARNING] 未知错误，可能的解决方案：
1) 重试推送
2) 检查网络连接
3) 退出脚本
```

**特性**：
- **自动重试**：最多3次推送尝试
- **用户选择**：每次失败后提供解决方案选项
- **不强制退出**：用户可选择继续或退出
- **智能识别**：自动识别不同类型的错误

## 📋 详细使用说明

### 🔑 SSH密钥设置 (`setup-github-ssh.sh`)

```bash
# 运行设置助手
./setup-github-ssh.sh

# 查看帮助
./setup-github-ssh.sh --help
```

**输出示例：**
```
🔑 GitHub SSH密钥设置助手
=========================

[INFO] 生成SSH密钥...
[SUCCESS] SSH密钥生成完成

[INFO] 您的SSH公钥内容：
==================================
ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGx... proreg@163.com
==================================

[SUCCESS] 公钥已复制到剪贴板

📋 GitHub SSH密钥设置步骤：
1. 复制上面显示的SSH公钥
2. 打开GitHub网站: https://github.com/settings/keys
...
```

### 📤 完整推送 (`push-to-github.sh`)

```bash
# 基本使用
./push-to-github.sh
```

**使用场景：**

#### 场景1：新项目
```bash
cd my-new-project
./push-to-github.sh
```
- 自动使用目录名作为仓库名
- 创建GitHub仓库（如果有GitHub CLI）
- 初始化Git并推送

#### 场景2：仓库名冲突
```
GitHub仓库 'common-name' 已存在
选择操作:
1) 使用现有仓库 (更新)
2) 输入新的项目名称
请选择 [1-2]: 2
请输入新的项目名称: my-unique-name
```

#### 场景3：更新现有项目
```bash
cd existing-project
./push-to-github.sh
```
- 检测现有仓库
- 提示输入提交信息
- 自动处理合并冲突

### ⚡ 快速推送 (`quick-push.sh`)

```bash
# 直接指定提交信息
./quick-push.sh "添加新功能"

# 交互式输入（回车使用默认时间戳）
./quick-push.sh

# 使用默认提交信息（时间戳）
./quick-push.sh ""
```

### 🧹 配置清理工具 (`clean-github-config.sh`)

```bash
# 运行清理工具
./clean-github-config.sh

# 查看帮助
./clean-github-config.sh --help
```

**使用场景：**
- 之前使用过其他GitHub账户
- 远程仓库配置混乱
- 需要彻底重置GitHub配置
- 多账户切换

**清理内容：**
- 全局Git用户配置
- 本地仓库远程配置
- SSH known_hosts中的GitHub条目
- Git凭据存储
- 分支跟踪配置

**输出示例：**
```
⚡ 快速推送到GitHub
==================
[INFO] 开始快速推送...
[INFO] 添加文件...
[INFO] 提交更改: 添加新功能
[INFO] 推送到远程仓库 (main)...
[SUCCESS] 推送完成！
🔗 仓库地址: https://github.com/proregmao/my-project
```

## ⚙️ 配置说明

### 默认配置
所有脚本使用统一的默认配置：

```bash
DEFAULT_EMAIL="proreg@163.com"
DEFAULT_USERNAME="proregmao"
```

### 自定义配置
如需修改，编辑对应脚本文件：

```bash
# 编辑主推送脚本
nano push-to-github.sh

# 编辑快速推送脚本
nano quick-push.sh

# 编辑SSH设置脚本
nano setup-github-ssh.sh
```

## 🛠️ 故障排除

### 问题1：权限被拒绝
```bash
chmod +x *.sh
```

### 问题2：SSH连接失败
```bash
# 重新运行SSH设置
./setup-github-ssh.sh

# 手动测试连接
ssh -T git@github.com
```

### 问题3：推送被拒绝
```bash
# 检查远程仓库状态
git remote -v

# 强制推送（谨慎使用）
git push --force-with-lease origin main
```

### 问题4：仓库名称无效
确保仓库名称只包含：
- 字母 (a-z, A-Z)
- 数字 (0-9)
- 点 (.)
- 下划线 (_)
- 连字符 (-)

## 📁 自动生成的文件

### .gitignore
脚本会自动创建包含常见忽略规则的.gitignore文件：

```gitignore
# 日志文件
*.log
logs/
*.pid

# 数据库文件
*.db
*.sqlite
*.sqlite3

# 临时文件
*.tmp
*.temp
.DS_Store
Thumbs.db

# 依赖目录
node_modules/
vendor/

# 构建输出
build/
dist/
*.exe
*.dll
*.so
*.dylib
```

## 🎉 成功示例

### 完整推送成功
```
🎉 项目已成功推送到GitHub！

📁 项目名称: my-awesome-project
🔗 仓库地址: https://github.com/proregmao/my-awesome-project
👤 用户名: proregmao
📧 邮箱: proreg@163.com

您可以访问上述链接查看您的项目
```

### 快速推送成功
```
[SUCCESS] 🎉 推送完成！
🔗 仓库地址: https://github.com/proregmao/my-project
```

## 💡 最佳实践

### 1. 首次使用流程
```bash
# 1. 设置SSH密钥
./setup-github-ssh.sh

# 2. 推送第一个项目
./push-to-github.sh

# 3. 后续快速更新
./quick-push.sh "更新内容"
```

### 2. 项目开发流程
```bash
# 开发阶段 - 频繁提交
./quick-push.sh "修复bug"
./quick-push.sh "添加功能"
./quick-push.sh "更新文档"

# 重大更新 - 使用完整脚本
./push-to-github.sh
```

### 3. 团队协作
```bash
# 推送前先拉取更新
git pull origin main

# 使用快速推送
./quick-push.sh "我的更改"
```

## 📞 技术支持

如遇问题，请检查：
1. 网络连接
2. SSH密钥配置
3. GitHub仓库权限
4. Git配置

**常用调试命令：**
```bash
# 检查Git配置
git config --list

# 检查SSH连接
ssh -T git@github.com

# 检查远程仓库
git remote -v

# 查看Git状态
git status
```

---

**🎯 快速开始：**
1. `./setup-github-ssh.sh` - 首次设置
2. `./push-to-github.sh` - 推送项目
3. `./quick-push.sh` - 日常更新

祝您使用愉快！🚀
