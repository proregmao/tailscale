#!/usr/bin/env node

const fs = require('fs');

function testTableStructure(filePath) {
    console.log(`\n🔍 测试表格结构: ${filePath}`);
    
    if (!fs.existsSync(filePath)) {
        console.log(`❌ 文件不存在: ${filePath}`);
        return false;
    }
    
    const content = fs.readFileSync(filePath, 'utf8');
    const lines = content.split('\n');
    
    let inScript = false;
    let inTable = false;
    let currentTr = null;
    let tdStack = [];
    let issues = [];
    
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
        if (line.includes('<table')) {
            inTable = true;
            console.log(`📊 表格开始: 第${lineNum}行`);
        }
        
        if (line.includes('</table>')) {
            inTable = false;
            console.log(`📊 表格结束: 第${lineNum}行`);
        }
        
        if (inTable) {
            // 检查 tr 标签
            if (line.includes('<tr>') || line.includes('<tr ')) {
                currentTr = { line: lineNum, tds: [] };
                console.log(`  📝 行开始: 第${lineNum}行`);
            }
            
            if (line.includes('</tr>')) {
                if (currentTr) {
                    console.log(`  📝 行结束: 第${lineNum}行 (包含 ${currentTr.tds.length} 个单元格)`);
                    currentTr = null;
                } else {
                    issues.push(`第${lineNum}行: 孤立的 </tr> 标签`);
                }
            }
            
            // 检查 td 标签
            const openTds = (line.match(/<td[^>]*>/g) || []).length;
            const closeTds = (line.match(/<\/td>/g) || []).length;
            
            if (openTds > 0) {
                for (let j = 0; j < openTds; j++) {
                    tdStack.push({ line: lineNum, type: 'open' });
                    if (currentTr) {
                        currentTr.tds.push({ line: lineNum, type: 'open' });
                    }
                }
                console.log(`    🔹 第${lineNum}行: ${openTds} 个 <td> 开放标签`);
            }
            
            if (closeTds > 0) {
                for (let j = 0; j < closeTds; j++) {
                    const lastOpen = tdStack.pop();
                    if (!lastOpen) {
                        issues.push(`第${lineNum}行: 多余的 </td> 标签`);
                        console.log(`    ❌ 第${lineNum}行: 多余的 </td> 标签`);
                    } else {
                        console.log(`    🔸 第${lineNum}行: </td> 关闭标签 (对应第${lastOpen.line}行)`);
                    }
                }
            }
        }
    }
    
    // 检查未关闭的 td 标签
    if (tdStack.length > 0) {
        tdStack.forEach(td => {
            issues.push(`第${td.line}行: 未关闭的 <td> 标签`);
            console.log(`❌ 第${td.line}行: 未关闭的 <td> 标签`);
        });
    }
    
    if (issues.length === 0) {
        console.log('✅ 表格结构正确');
        return true;
    } else {
        console.log(`❌ 发现 ${issues.length} 个问题:`);
        issues.forEach(issue => console.log(`  - ${issue}`));
        return false;
    }
}

// 测试 routes.html
const file = '/root/tailscale/headscale-ui/src/routes/routes.html/+page.svelte';
testTableStructure(file);
