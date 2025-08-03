#!/usr/bin/env node

const fs = require('fs');

// 需要修复的文件
const files = [
    'headscale-ui/src/routes/routes.html/+page.svelte',
    'headscale-ui/src/routes/ssh.html/+page.svelte', 
    'headscale-ui/src/routes/serve.html/+page.svelte'
];

function analyzeStructure(content) {
    const lines = content.split('\n');
    const stack = [];
    const issues = [];
    
    for (let i = 0; i < lines.length; i++) {
        const line = lines[i];
        const lineNum = i + 1;
        
        // 检查开放标签
        const openTags = line.match(/<(\w+)[^>]*(?<!\/)\>/g) || [];
        const closeTags = line.match(/<\/(\w+)>/g) || [];
        const selfCloseTags = line.match(/<\w+[^>]*\/>/g) || [];
        
        // 处理开放标签
        openTags.forEach(tag => {
            const tagName = tag.match(/<(\w+)/)[1];
            if (!['input', 'img', 'br', 'hr', 'meta', 'link'].includes(tagName)) {
                stack.push({ tag: tagName, line: lineNum, content: tag });
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
                    content: tag
                });
            } else if (lastOpen.tag !== tagName) {
                issues.push({
                    type: 'mismatch',
                    line: lineNum,
                    expected: lastOpen.tag,
                    found: tagName,
                    openLine: lastOpen.line
                });
                // 把不匹配的标签放回栈中
                stack.push(lastOpen);
            }
        });
        
        // 检查 Svelte 块
        const openBlocks = line.match(/\{#\w+/g) || [];
        const closeBlocks = line.match(/\{\/\w+\}/g) || [];
        
        openBlocks.forEach(block => {
            const blockType = block.match(/\{#(\w+)/)[1];
            stack.push({ tag: `svelte:${blockType}`, line: lineNum, content: block });
        });
        
        closeBlocks.forEach(block => {
            const blockType = block.match(/\{\/(\w+)\}/)[1];
            const lastOpen = stack.pop();
            
            if (!lastOpen || lastOpen.tag !== `svelte:${blockType}`) {
                issues.push({
                    type: 'svelte_mismatch',
                    line: lineNum,
                    expected: lastOpen ? lastOpen.tag : 'none',
                    found: `svelte:${blockType}`
                });
                if (lastOpen) stack.push(lastOpen);
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
    
    return { issues, stack };
}

function fixStructure(filePath) {
    console.log(`\n🔧 修复文件: ${filePath}`);
    
    if (!fs.existsSync(filePath)) {
        console.log(`❌ 文件不存在: ${filePath}`);
        return false;
    }
    
    let content = fs.readFileSync(filePath, 'utf8');
    const { issues } = analyzeStructure(content);
    
    if (issues.length === 0) {
        console.log('✅ 文件结构正确');
        return true;
    }
    
    console.log(`发现 ${issues.length} 个问题:`);
    issues.forEach(issue => {
        console.log(`  - 第${issue.line}行: ${issue.type} - ${issue.tag || issue.found}`);
    });
    
    // 修复策略：
    // 1. 删除多余的关闭标签
    // 2. 为未关闭的标签添加关闭标签
    
    let lines = content.split('\n');
    let modified = false;
    
    // 删除多余的关闭标签
    issues.filter(i => i.type === 'extra_close').forEach(issue => {
        const lineIndex = issue.line - 1;
        const line = lines[lineIndex];
        const newLine = line.replace(new RegExp(`</${issue.tag}>`, 'g'), '');
        if (newLine !== line) {
            lines[lineIndex] = newLine;
            modified = true;
            console.log(`  ✓ 删除第${issue.line}行的多余 </${issue.tag}>`);
        }
    });
    
    // 为未关闭的标签添加关闭标签
    const unclosedTags = issues.filter(i => i.type === 'unclosed');
    if (unclosedTags.length > 0) {
        // 在文件末尾的 {/if} 之前添加关闭标签
        const lastLineIndex = lines.length - 1;
        let insertIndex = lastLineIndex;
        
        // 找到最后的 {/if} 位置
        for (let i = lines.length - 1; i >= 0; i--) {
            if (lines[i].trim() === '{/if}') {
                insertIndex = i;
                break;
            }
        }
        
        // 按照栈的顺序（后进先出）添加关闭标签
        unclosedTags.reverse().forEach(issue => {
            if (issue.tag.startsWith('svelte:')) {
                const blockType = issue.tag.replace('svelte:', '');
                lines.splice(insertIndex, 0, `\t{/${blockType}}`);
                console.log(`  ✓ 添加 {/${blockType}} 在第${insertIndex + 1}行`);
            } else {
                lines.splice(insertIndex, 0, `\t</${issue.tag}>`);
                console.log(`  ✓ 添加 </${issue.tag}> 在第${insertIndex + 1}行`);
            }
            insertIndex++;
            modified = true;
        });
    }
    
    if (modified) {
        fs.writeFileSync(filePath, lines.join('\n'));
        console.log('✅ 文件已修复');
        return true;
    } else {
        console.log('⚠️ 无法自动修复');
        return false;
    }
}

async function main() {
    console.log('🔧 开始修复 Svelte 文件结构...\n');
    
    let allFixed = true;
    for (const file of files) {
        const fixed = fixStructure(file);
        if (!fixed) allFixed = false;
    }
    
    if (allFixed) {
        console.log('\n🎉 所有文件修复完成！');
    } else {
        console.log('\n⚠️ 部分文件需要手动修复');
    }
}

if (require.main === module) {
    main();
}
