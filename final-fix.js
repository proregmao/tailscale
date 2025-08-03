#!/usr/bin/env node

const fs = require('fs');

function finalFix(filePath) {
    console.log(`\n🔧 最终修复: ${filePath}`);
    
    if (!fs.existsSync(filePath)) {
        console.log(`❌ 文件不存在: ${filePath}`);
        return false;
    }
    
    let content = fs.readFileSync(filePath, 'utf8');
    let modified = false;
    
    // 1. 修复可能导致 td 错误的 {#if} 块问题
    // 确保 {#if} 块不会破坏表格结构
    const ifInTdPattern = /(<td[^>]*>[\s\S]*?)\{#if([^}]*)\}([\s\S]*?)\{:else\}([\s\S]*?)\{\/if\}([\s\S]*?<\/td>)/g;
    content = content.replace(ifInTdPattern, (match, tdStart, condition, ifContent, elseContent, tdEnd) => {
        // 确保 if 块内容正确格式化
        const fixed = `${tdStart}{#if${condition}}${ifContent.trim()}{:else}${elseContent.trim()}{/if}${tdEnd}`;
        if (fixed !== match) {
            console.log(`🔧 修复 td 内的 if 块`);
            modified = true;
        }
        return fixed;
    });
    
    // 2. 修复可能的 span 标签问题
    // 确保 span 标签正确关闭
    const unclosedSpanPattern = /<span[^>]*>([^<]*(?:<(?!\/span>)[^<]*)*[^<]*)(?!<\/span>)/g;
    content = content.replace(unclosedSpanPattern, (match, content) => {
        if (!match.includes('</span>')) {
            console.log(`🔧 修复未关闭的 span 标签`);
            modified = true;
            return match + '</span>';
        }
        return match;
    });
    
    // 3. 修复可能的表格行结构问题
    // 确保每个 tr 都有正确的结构
    const trPattern = /<tr[^>]*>([\s\S]*?)<\/tr>/g;
    content = content.replace(trPattern, (match, trContent) => {
        // 计算 td 的开放和关闭标签
        const openTds = (trContent.match(/<td[^>]*>/g) || []).length;
        const closeTds = (trContent.match(/<\/td>/g) || []).length;
        
        if (openTds !== closeTds) {
            console.log(`🔧 修复 tr 内的 td 标签不匹配: ${openTds} 开放, ${closeTds} 关闭`);
            modified = true;
            
            // 如果缺少关闭标签，在 tr 结束前添加
            if (openTds > closeTds) {
                const missing = openTds - closeTds;
                let fixed = trContent;
                for (let i = 0; i < missing; i++) {
                    fixed += '\n\t\t\t\t\t</td>';
                }
                return `<tr>${fixed}\n\t\t\t\t</tr>`;
            }
        }
        return match;
    });
    
    // 4. 清理多余的空白和格式问题
    content = content.replace(/\n\s*\n\s*\n/g, '\n\n');
    content = content.replace(/\t+\n/g, '\n');
    
    // 5. 确保文件以正确的格式结束
    content = content.replace(/\s+$/, '\n');
    
    if (modified) {
        fs.writeFileSync(filePath, content);
        console.log('✅ 最终修复完成');
        return true;
    } else {
        console.log('✅ 无需最终修复');
        return true;
    }
}

// 需要修复的文件
const files = [
    '/root/tailscale/headscale-ui/src/routes/routes.html/+page.svelte',
    '/root/tailscale/headscale-ui/src/routes/ssh.html/+page.svelte', 
    '/root/tailscale/headscale-ui/src/routes/serve.html/+page.svelte'
];

async function main() {
    console.log('🔧 开始最终修复...\n');
    
    let allFixed = true;
    for (const file of files) {
        const fixed = finalFix(file);
        if (!fixed) allFixed = false;
    }
    
    if (allFixed) {
        console.log('\n🎉 最终修复完成！');
    } else {
        console.log('\n⚠️ 部分文件需要手动修复');
    }
}

if (require.main === module) {
    main();
}
