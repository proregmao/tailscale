#!/usr/bin/env node

const fs = require('fs');
const path = require('path');

// 需要修复的文件列表
const files = [
    'src/routes/routes.html/+page.svelte',
    'src/routes/sdk.html/+page.svelte',
    'src/routes/serve.html/+page.svelte',
    'src/routes/ssh.html/+page.svelte'
];

function analyzeStructure(content) {
    const lines = content.split('\n');
    const stack = [];
    const issues = [];
    
    for (let i = 0; i < lines.length; i++) {
        const line = lines[i];
        const lineNum = i + 1;
        
        // 检查 HTML 标签
        const openTags = line.match(/<(\w+)(?:\s[^>]*)?(?<!\/)\s*>/g) || [];
        const closeTags = line.match(/<\/(\w+)\s*>/g) || [];
        const selfClosingTags = line.match(/<(\w+)(?:\s[^>]*)?\/\s*>/g) || [];
        
        // 检查 Svelte 块
        const openBlocks = line.match(/\{#(\w+)(?:\s[^}]*)?\}/g) || [];
        const closeBlocks = line.match(/\{\/(\w+)\}/g) || [];
        
        // 处理开放标签
        openTags.forEach(tag => {
            const tagName = tag.match(/<(\w+)/)[1];
            if (!['img', 'input', 'br', 'hr', 'meta', 'link', 'area', 'base', 'col', 'embed', 'source', 'track', 'wbr'].includes(tagName)) {
                stack.push({ type: 'tag', name: tagName, line: lineNum });
            }
        });
        
        // 处理开放块
        openBlocks.forEach(block => {
            const blockName = block.match(/\{#(\w+)/)[1];
            stack.push({ type: 'block', name: blockName, line: lineNum });
        });
        
        // 处理关闭标签
        closeTags.forEach(tag => {
            const tagName = tag.match(/<\/(\w+)/)[1];
            const lastItem = stack[stack.length - 1];
            
            if (!lastItem || lastItem.type !== 'tag' || lastItem.name !== tagName) {
                issues.push({
                    line: lineNum,
                    type: 'tag_mismatch',
                    expected: lastItem ? lastItem.name : 'none',
                    found: tagName,
                    content: line.trim()
                });
            } else {
                stack.pop();
            }
        });
        
        // 处理关闭块
        closeBlocks.forEach(block => {
            const blockName = block.match(/\{\/(\w+)/)[1];
            const lastItem = stack[stack.length - 1];
            
            if (!lastItem || lastItem.type !== 'block' || lastItem.name !== blockName) {
                issues.push({
                    line: lineNum,
                    type: 'block_mismatch',
                    expected: lastItem ? lastItem.name : 'none',
                    found: blockName,
                    content: line.trim()
                });
            } else {
                stack.pop();
            }
        });
    }
    
    // 检查未关闭的元素
    stack.forEach(item => {
        issues.push({
            line: item.line,
            type: 'unclosed',
            name: item.name,
            elementType: item.type
        });
    });
    
    return { issues, stack };
}

function main() {
    console.log('🔍 分析 Svelte 文件结构...\n');
    
    files.forEach(file => {
        if (!fs.existsSync(file)) {
            console.log(`❌ 文件不存在: ${file}`);
            return;
        }
        
        console.log(`📄 分析文件: ${file}`);
        const content = fs.readFileSync(file, 'utf8');
        const { issues, stack } = analyzeStructure(content);
        
        if (issues.length === 0) {
            console.log('✅ 结构正确\n');
        } else {
            console.log(`❌ 发现 ${issues.length} 个问题:`);
            issues.forEach(issue => {
                switch (issue.type) {
                    case 'tag_mismatch':
                        console.log(`  第${issue.line}行: 标签不匹配 - 期望 </${issue.expected}>, 发现 </${issue.found}>`);
                        console.log(`    内容: ${issue.content}`);
                        break;
                    case 'block_mismatch':
                        console.log(`  第${issue.line}行: 块不匹配 - 期望 {/${issue.expected}}, 发现 {/${issue.found}}`);
                        console.log(`    内容: ${issue.content}`);
                        break;
                    case 'unclosed':
                        console.log(`  第${issue.line}行: 未关闭的${issue.elementType} - ${issue.name}`);
                        break;
                }
            });
            console.log();
        }
    });
}

if (require.main === module) {
    main();
}
