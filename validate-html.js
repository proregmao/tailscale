#!/usr/bin/env node

const fs = require('fs');

function validateHTMLStructure(filePath) {
    console.log(`\n🔍 验证 HTML 结构: ${filePath}`);
    
    if (!fs.existsSync(filePath)) {
        console.log(`❌ 文件不存在: ${filePath}`);
        return false;
    }
    
    const content = fs.readFileSync(filePath, 'utf8');
    const lines = content.split('\n');
    
    const stack = [];
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
        
        // 如果在 script 标签内，跳过 HTML 标签检查
        if (inScript) {
            continue;
        }
        
        // 检查 HTML 标签
        const openTags = line.match(/<(\w+)[^>]*(?<!\/)\>/g) || [];
        const closeTags = line.match(/<\/(\w+)>/g) || [];
        
        // 处理开放标签
        openTags.forEach(tag => {
            const tagName = tag.match(/<(\w+)/)[1];
            // 跳过自闭合标签
            if (!['input', 'img', 'br', 'hr', 'meta', 'link', 'option'].includes(tagName)) {
                stack.push({ tag: tagName, line: lineNum, content: tag.trim() });
            }
        });
        
        // 处理关闭标签
        closeTags.forEach(tag => {
            const tagName = tag.match(/<\/(\w+)>/)[1];
            const lastOpen = stack.pop();
            
            if (!lastOpen) {
                issues.push({
                    type: 'extra_close',
                    line: lineNum,
                    tag: tagName,
                    content: line.trim()
                });
            } else if (lastOpen.tag !== tagName) {
                issues.push({
                    type: 'mismatch',
                    line: lineNum,
                    expected: lastOpen.tag,
                    found: tagName,
                    openLine: lastOpen.line,
                    content: line.trim()
                });
                // 把不匹配的标签放回栈中
                stack.push(lastOpen);
            }
        });
    }
    
    // 检查未关闭的标签
    stack.forEach(item => {
        issues.push({
            type: 'unclosed',
            line: item.line,
            tag: item.tag,
            content: item.content
        });
    });
    
    if (issues.length === 0) {
        console.log('✅ HTML 结构正确');
        return true;
    } else {
        console.log(`❌ 发现 ${issues.length} 个问题:`);
        issues.forEach(issue => {
            switch (issue.type) {
                case 'extra_close':
                    console.log(`  🔴 第${issue.line}行: 多余的关闭标签 </${issue.tag}>`);
                    console.log(`      内容: ${issue.content}`);
                    break;
                case 'mismatch':
                    console.log(`  🟡 第${issue.line}行: 标签不匹配`);
                    console.log(`      期望: </${issue.expected}> (第${issue.openLine}行开始)`);
                    console.log(`      实际: </${issue.found}>`);
                    console.log(`      内容: ${issue.content}`);
                    break;
                case 'unclosed':
                    console.log(`  🟠 第${issue.line}行: 未关闭的标签 <${issue.tag}>`);
                    console.log(`      内容: ${issue.content}`);
                    break;
            }
        });
        return false;
    }
}

// 需要验证的文件
const files = [
    '/root/tailscale/headscale-ui/src/routes/routes.html/+page.svelte',
    '/root/tailscale/headscale-ui/src/routes/ssh.html/+page.svelte', 
    '/root/tailscale/headscale-ui/src/routes/serve.html/+page.svelte'
];

async function main() {
    console.log('🔍 开始验证 HTML 结构...\n');
    
    let allValid = true;
    for (const file of files) {
        const valid = validateHTMLStructure(file);
        if (!valid) allValid = false;
    }
    
    if (allValid) {
        console.log('\n🎉 所有文件 HTML 结构正确！');
    } else {
        console.log('\n⚠️ 部分文件存在 HTML 结构问题');
    }
}

if (require.main === module) {
    main();
}
