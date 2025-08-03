# GitHub推送脚本使用说明

## 📋 功能特性

这个脚本可以帮助您自动将本地项目推送到GitHub，支持以下功能：

- ✅ **自动检测项目名称**：使用当前目录名作为默认项目名
- ✅ **智能仓库管理**：自动检测GitHub仓库是否存在
- ✅ **新建/更新模式**：支持新建仓库和更新现有仓库
- ✅ **自动配置Git**：设置默认用户信息
- ✅ **智能文件管理**：自动创建.gitignore文件
- ✅ **分支管理**：支持多分支推送
- ✅ **冲突处理**：自动处理合并冲突
- ✅ **GitHub CLI集成**：支持自动创建仓库

## 🚀 快速开始

### 1. 基本使用

```bash
# 在项目目录下运行
./push-to-github.sh
```

### 2. 首次使用

脚本会自动：
1. 检查并安装必要工具
2. 配置Git用户信息（邮箱：proreg@163.com，用户名：proregmao）
3. 初始化Git仓库（如果不存在）
4. 检查GitHub仓库状态
5. 创建.gitignore文件（如果不存在）
6. 添加所有文件到Git
7. 提交更改
8. 推送到GitHub

## 📖 使用场景

### 场景1：新项目首次推送

```bash
cd my-new-project
./push-to-github.sh
```

**脚本行为：**
- 使用目录名 `my-new-project` 作为GitHub仓库名
- 如果仓库不存在，提示创建新仓库
- 执行首次推送

### 场景2：项目名称冲突

如果GitHub上已存在同名仓库：

```
GitHub仓库 'my-project' 已存在
选择操作:
1) 使用现有仓库 (更新)
2) 输入新的项目名称
请选择 [1-2]: 
```

### 场景3：更新现有项目

```bash
cd existing-project
./push-to-github.sh
```

**脚本行为：**
- 检测到现有远程仓库
- 提示输入提交信息
- 自动拉取远程更新
- 推送本地更改

## ⚙️ 配置说明

### 默认配置

```bash
DEFAULT_EMAIL="proreg@163.com"
DEFAULT_USERNAME="proregmao"
GITHUB_BASE_URL="https://github.com"
```

### 自定义配置

如需修改默认配置，编辑脚本文件：

```bash
nano push-to-github.sh
```

## 📁 自动生成的.gitignore

脚本会自动创建包含以下内容的.gitignore文件：

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

# IDE文件
.vscode/
.idea/
*.swp
*.swo
*~

# 环境配置
.env
.env.local
.env.production

# 缓存
.cache/
*.cache
```

## 🛠️ 依赖工具

### 必需工具

- **Git**：版本控制
- **curl**：网络请求

### 可选工具

- **GitHub CLI (gh)**：自动创建仓库
  ```bash
  # 安装GitHub CLI
  # macOS
  brew install gh
  
  # Ubuntu/Debian
  sudo apt install gh
  
  # 认证
  gh auth login
  ```

## 🔧 高级功能

### 1. 分支管理

脚本支持多分支推送：
- 自动检测当前分支
- 首次推送时切换到main分支
- 更新时保持当前分支

### 2. 冲突处理

自动处理合并冲突：
- 推送前自动拉取远程更新
- 使用rebase模式避免不必要的合并提交
- 冲突时提供清晰的错误信息

### 3. 智能检测

- 检测现有Git仓库
- 检测远程仓库配置
- 检测GitHub仓库存在性

## 🎯 使用示例

### 示例1：新建React项目

```bash
npx create-react-app my-react-app
cd my-react-app
/path/to/push-to-github.sh
```

### 示例2：更新现有项目

```bash
cd my-existing-project
# 做一些修改
echo "新功能" >> README.md
/path/to/push-to-github.sh
# 输入提交信息：添加新功能说明
```

### 示例3：处理名称冲突

```bash
cd common-name-project
/path/to/push-to-github.sh
# 选择选项2，输入新名称：my-unique-project-name
```

## 🚨 注意事项

1. **SSH密钥配置**：确保已配置GitHub SSH密钥
2. **权限问题**：确保脚本有执行权限
3. **网络连接**：需要稳定的网络连接
4. **大文件处理**：大文件建议使用Git LFS

## 🔍 故障排除

### 问题1：权限被拒绝

```bash
chmod +x push-to-github.sh
```

### 问题2：SSH密钥未配置

```bash
ssh-keygen -t ed25519 -C "proreg@163.com"
cat ~/.ssh/id_ed25519.pub
# 将公钥添加到GitHub
```

### 问题3：推送失败

检查网络连接和GitHub状态：
```bash
ssh -T git@github.com
```

## 📞 支持

如有问题，请检查：
1. 网络连接
2. Git配置
3. GitHub SSH密钥
4. 仓库权限

## 🎉 成功示例

脚本成功运行后会显示：

```
🎉 项目已成功推送到GitHub！

📁 项目名称: my-project
🔗 仓库地址: https://github.com/proregmao/my-project
👤 用户名: proregmao
📧 邮箱: proreg@163.com

您可以访问上述链接查看您的项目
```
