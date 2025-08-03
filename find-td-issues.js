#!/usr/bin/env node

const fs = require('fs');

function findTdIssues(filePath) {
    console.log(`\n🔍 检查 TD 标签问题: ${filePath}`);
    
    if (!fs.existsSync(filePath)) {
        console.log(`❌ 文件不存在: ${filePath}`);
        return false;
    }
    
    const content = fs.readFileSync(filePath, 'utf8');
    const lines = content.split('\n');
    
    const tdStack = [];
    const issues = [];
    let inScript = false;
    
    for (let i = 0; i < lines.length; i++) {
        const line = lines[i];
        const lineNum = i + 1;
        
        // 检查是否在 script 标签内
        if (line.includes('<script')) {
            inScript = true;
            continue;
        }
        if (line.includes('</script>')) {
            inScript = false;
            continue;
        }
        
        // 如果在 script 标签内，跳过
        if (inScript) {
            continue;
        }
        
        // 检查 td 标签
        const openTds = line.match(/<td[^>]*>/g) || [];
        const closeTds = line.match(/<\/td>/g) || [];
        
        // 处理开放的 td 标签
        openTds.forEach(tag => {
            tdStack.push({ line: lineNum, content: line.trim() });
        });
        
        // 处理关闭的 td 标签
        closeTds.forEach(tag => {
            const lastOpen = tdStack.pop();
            
            if (!lastOpen) {
                issues.push({
                    type: 'extra_close_td',
                    line: lineNum,
                    content: line.trim()
                });
                console.log(`🔴 第${lineNum}行: 多余的 </td> 标签`);
                console.log(`    内容: ${line.trim()}`);
            }
        });
    }
    
    // 检查未关闭的 td 标签
    tdStack.forEach(item => {
        issues.push({
            type: 'unclosed_td',
            line: item.line,
            content: item.content
        });
        console.log(`🟠 第${item.line}行: 未关闭的 <td> 标签`);
        console.log(`    内容: ${item.content}`);
    });
    
    if (issues.length === 0) {
        console.log('✅ TD 标签结构正确');
        return true;
    } else {
        console.log(`❌ 发现 ${issues.length} 个 TD 标签问题`);
        return false;
    }
}

// 需要检查的文件
const files = [
    '/root/tailscale/headscale-ui/src/routes/routes.html/+page.svelte',
    '/root/tailscale/headscale-ui/src/routes/ssh.html/+page.svelte', 
    '/root/tailscale/headscale-ui/src/routes/serve.html/+page.svelte'
];

async function main() {
    console.log('🔍 开始检查 TD 标签问题...\n');
    
    let allValid = true;
    for (const file of files) {
        const valid = findTdIssues(file);
        if (!valid) allValid = false;
    }
    
    if (allValid) {
        console.log('\n🎉 所有文件 TD 标签正确！');
    } else {
        console.log('\n⚠️ 部分文件存在 TD 标签问题');
    }
}

if (require.main === module) {
    main();
}
