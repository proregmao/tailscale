#!/usr/bin/env node

const fs = require('fs');

function validateTableStructure(filePath) {
    console.log(`\n🔍 验证表格结构: ${filePath}`);
    
    if (!fs.existsSync(filePath)) {
        console.log(`❌ 文件不存在: ${filePath}`);
        return false;
    }
    
    const content = fs.readFileSync(filePath, 'utf8');
    const lines = content.split('\n');
    
    const stack = [];
    const issues = [];
    let inScript = false;
    let currentTable = null;
    
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
        
        // 检查表格相关标签
        const tableOpenTags = line.match(/<(table|thead|tbody|tr|td|th)[^>]*>/g) || [];
        const tableCloseTags = line.match(/<\/(table|thead|tbody|tr|td|th)>/g) || [];
        
        // 处理开放标签
        tableOpenTags.forEach(tag => {
            const tagName = tag.match(/<(\w+)/)[1];
            stack.push({ tag: tagName, line: lineNum, content: line.trim() });
            
            if (tagName === 'table') {
                currentTable = { line: lineNum, rows: 0, cells: [] };
            } else if (tagName === 'tr' && currentTable) {
                currentTable.rows++;
                currentTable.currentRow = [];
            } else if ((tagName === 'td' || tagName === 'th') && currentTable && currentTable.currentRow) {
                currentTable.currentRow.push({ tag: tagName, line: lineNum });
            }
        });
        
        // 处理关闭标签
        tableCloseTags.forEach(tag => {
            const tagName = tag.match(/<\/(\w+)>/)[1];
            const lastOpen = stack.pop();
            
            if (!lastOpen) {
                issues.push({
                    type: 'extra_close',
                    line: lineNum,
                    tag: tagName,
                    content: line.trim()
                });
                console.log(`🔴 第${lineNum}行: 多余的 </${tagName}> 标签`);
                console.log(`    内容: ${line.trim()}`);
            } else if (lastOpen.tag !== tagName) {
                issues.push({
                    type: 'mismatch',
                    line: lineNum,
                    expected: lastOpen.tag,
                    found: tagName,
                    openLine: lastOpen.line,
                    content: line.trim()
                });
                console.log(`🟡 第${lineNum}行: 标签不匹配`);
                console.log(`    期望: </${lastOpen.tag}> (第${lastOpen.line}行开始)`);
                console.log(`    实际: </${tagName}>`);
                console.log(`    内容: ${line.trim()}`);
                // 把不匹配的标签放回栈中
                stack.push(lastOpen);
            }
            
            if (tagName === 'tr' && currentTable && currentTable.currentRow) {
                currentTable.cells.push(currentTable.currentRow);
                currentTable.currentRow = null;
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
        console.log(`🟠 第${item.line}行: 未关闭的 <${item.tag}> 标签`);
        console.log(`    内容: ${item.content}`);
    });
    
    if (issues.length === 0) {
        console.log('✅ 表格结构正确');
        return true;
    } else {
        console.log(`❌ 发现 ${issues.length} 个表格结构问题`);
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
    console.log('🔍 开始验证表格结构...\n');
    
    let allValid = true;
    for (const file of files) {
        const valid = validateTableStructure(file);
        if (!valid) allValid = false;
    }
    
    if (allValid) {
        console.log('\n🎉 所有文件表格结构正确！');
    } else {
        console.log('\n⚠️ 部分文件存在表格结构问题');
    }
}

if (require.main === module) {
    main();
}
