# 🚀 Vuestic Admin 快速参考手册

## ⚡ 快速启动

```bash
# 1. 安装依赖
yarn install

# 2. 启动开发服务器
yarn dev
# 访问: http://localhost:5173/

# 3. 快速测试
node quick-test.js
```

## 🧪 测试命令

```bash
# 代码质量检查
yarn lint                    # ESLint 检查
yarn vue-tsc --noEmit       # TypeScript 检查

# 构建测试
yarn build:ci               # 生产构建
yarn start:ci               # 启动生产服务器 (端口3000)

# Playwright 功能测试
node quick-test.js           # 快速测试 (3页面)
node run-tests.js full      # 完整测试 (9页面)
node run-tests.js single "Users" "/users"  # 单页面测试
node run-tests.js responsive # 响应式测试
```

## 🔧 故障排除

### Playwright 问题
```bash
# 浏览器锁定
pkill -f "playwright"
rm -rf /root/.cache/ms-playwright
npx playwright install chromium

# 权限问题
chmod +x *.js
```

### 端口占用
```bash
# 查找占用进程
sudo lsof -i :5173
sudo lsof -i :3000

# 终止进程
sudo kill -9 <PID>
```

### 依赖问题
```bash
# 清理重装
rm -rf node_modules yarn.lock
yarn install

# 更新浏览器数据
npx update-browserslist-db@latest
```

## 📊 测试结果

### 成功标准
- ✅ ESLint: 0 errors, 0 warnings
- ✅ TypeScript: 无类型错误
- ✅ 构建: 成功生成 dist/
- ✅ 功能测试: 9/9 页面通过 (100%)

### 生成文件
- `test-report.json` - 详细测试报告
- `test-report.html` - 可视化报告
- `screenshot-*.png` - 页面截图

## 🎯 测试覆盖

### 页面列表
1. Dashboard (`/dashboard`)
2. Users (`/users`)
3. Projects (`/projects`)
4. Payments (`/payments`)
5. Billing (`/billing`)
6. Pricing Plans (`/pricing-plans`)
7. FAQ (`/faq`)
8. Settings (`/settings`)
9. Preferences (`/preferences`)

### 测试类型
- 📄 页面加载测试
- 🖱️ 交互元素测试
- 📱 响应式设计测试
- 📸 自动截图记录
- 🐛 JavaScript 错误检测

## 🔗 相关文档

- [完整安装指南](./installation-and-testing-guide.md)
- [Playwright 测试指南](../PLAYWRIGHT_TESTING_GUIDE.md)
- [项目 README](../README.md)

---

*快速参考 v1.0 | 更新时间: 2025-08-01*
