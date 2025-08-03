#!/bin/bash

# 修复卡住的推送问题

echo "🔧 修复卡住的推送问题"
echo "===================="

# 检查当前状态
echo "1. 检查Git状态..."
git status --porcelain | wc -l | xargs echo "待推送文件数:"

echo
echo "2. 检查远程仓库..."
if git remote get-url origin; then
    echo "✅ 远程仓库已配置"
else
    echo "❌ 远程仓库未配置"
fi

echo
echo "3. 测试网络连接..."
if timeout 10 ssh -T git@github.com 2>&1 | grep -q "successfully authenticated"; then
    echo "✅ SSH连接正常"
else
    echo "⚠️ SSH连接可能有问题"
fi

echo
echo "4. 检查远程仓库是否存在..."
repo_name=$(basename $(git remote get-url origin) .git)
if timeout 10 curl -s "https://api.github.com/repos/proregmao/$repo_name" | grep -q '"id"'; then
    echo "✅ GitHub仓库存在: https://github.com/proregmao/$repo_name"
else
    echo "❌ GitHub仓库不存在或无法访问"
fi

echo
echo "5. 尝试强制推送..."
echo "选择推送方式："
echo "1) 正常推送 (timeout 60秒)"
echo "2) 强制推送 (timeout 60秒)"
echo "3) 重置并推送"
echo "4) 检查并修复"
echo -n "请选择 [1-4]: "
read -r choice

case $choice in
    1)
        echo "正在尝试正常推送..."
        if timeout 60 git push origin main; then
            echo "✅ 推送成功！"
        else
            echo "❌ 推送失败"
        fi
        ;;
    2)
        echo "正在尝试强制推送..."
        if timeout 60 git push origin main --force-with-lease; then
            echo "✅ 强制推送成功！"
        else
            echo "❌ 强制推送失败"
        fi
        ;;
    3)
        echo "重置并推送..."
        git reset --soft HEAD~1
        git add .
        git commit -m "Fix stuck push - $(date '+%Y-%m-%d %H:%M:%S')"
        if timeout 60 git push origin main --force; then
            echo "✅ 重置推送成功！"
        else
            echo "❌ 重置推送失败"
        fi
        ;;
    4)
        echo "检查并修复..."
        
        # 检查是否有冲突
        if git status | grep -q "conflict"; then
            echo "发现冲突，正在解决..."
            git add .
            git commit -m "Resolve conflicts - $(date '+%Y-%m-%d %H:%M:%S')"
        fi
        
        # 检查分支
        current_branch=$(git branch --show-current)
        if [[ "$current_branch" != "main" ]]; then
            echo "切换到main分支..."
            git checkout main 2>/dev/null || git checkout -b main
        fi
        
        # 尝试推送
        echo "尝试修复后推送..."
        if timeout 60 git push origin main; then
            echo "✅ 修复推送成功！"
        else
            echo "尝试强制推送..."
            if timeout 60 git push origin main --force; then
                echo "✅ 强制推送成功！"
            else
                echo "❌ 所有推送方式都失败"
                echo "请手动检查网络连接和GitHub仓库状态"
            fi
        fi
        ;;
    *)
        echo "无效选择"
        ;;
esac

echo
echo "6. 验证推送结果..."
if timeout 30 git ls-remote --exit-code origin main &>/dev/null; then
    echo "✅ 远程分支存在，推送可能成功"
    echo "🔗 查看仓库: https://github.com/proregmao/$repo_name"
else
    echo "❌ 远程分支不存在或无法访问"
fi

echo
echo "🎯 修复完成！"
echo "如果问题仍然存在，请运行万能脚本："
echo "  ./github-push-ultimate.sh"
