#!/bin/bash

# 测试万能脚本的功能

echo "🧪 测试万能脚本功能"
echo "=================="

# 测试帮助功能
echo "1. 测试帮助功能..."
./github-push-ultimate.sh --help | head -5

echo
echo "2. 测试脚本是否可执行..."
if [[ -x "./github-push-ultimate.sh" ]]; then
    echo "✅ 脚本可执行"
else
    echo "❌ 脚本不可执行"
fi

echo
echo "3. 检查脚本语法..."
if bash -n ./github-push-ultimate.sh; then
    echo "✅ 语法检查通过"
else
    echo "❌ 语法错误"
fi

echo
echo "4. 检查Git状态..."
git status --porcelain | wc -l | xargs echo "待提交文件数:"

echo
echo "5. 检查远程仓库配置..."
git remote -v

echo
echo "🎯 万能脚本功能验证完成！"
echo
echo "使用方法："
echo "  ./github-push-ultimate.sh              # 完整模式"
echo "  ./github-push-ultimate.sh quick        # 快速模式"
echo "  ./github-push-ultimate.sh \"提交信息\"   # 快速推送"
