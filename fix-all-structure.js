#!/usr/bin/env node

const fs = require('fs');

function fixAllStructure(filePath) {
    console.log(`\n🔧 修复所有结构问题: ${filePath}`);
    
    if (!fs.existsSync(filePath)) {
        console.log(`❌ 文件不存在: ${filePath}`);
        return false;
    }
    
    let content = fs.readFileSync(filePath, 'utf8');
    let modified = false;
    
    // 1. 确保所有的 button 标签都正确关闭
    // 查找没有关闭的 button 标签模式
    const buttonPattern = /<button[^>]*>\s*([^<]*)\s*(?!<\/button>)/g;
    content = content.replace(buttonPattern, (match, text) => {
        if (!match.includes('</button>')) {
            console.log(`🔧 修复 button 标签: ${text.trim()}`);
            modified = true;
            return match + '\n\t\t\t\t\t\t</button>';
        }
        return match;
    });
    
    // 2. 修复 select 和 option 标签结构
    // 确保 select 标签正确关闭
    content = content.replace(/<select([^>]*)>\s*(<option[^>]*>[^<]*<\/option>\s*)*\s*(?!<\/select>)/g, (match) => {
        if (!match.includes('</select>')) {
            console.log(`🔧 修复 select 标签`);
            modified = true;
            return match + '\n\t\t\t\t\t</select>';
        }
        return match;
    });
    
    // 3. 修复 textarea 标签
    content = content.replace(/<textarea([^>]*)>\s*([^<]*)\s*(?!<\/textarea>)/g, (match, attrs, text) => {
        if (!match.includes('</textarea>')) {
            console.log(`🔧 修复 textarea 标签`);
            modified = true;
            return `<textarea${attrs}>${text}</textarea>`;
        }
        return match;
    });
    
    // 4. 修复常见的标签嵌套问题
    // 移除多余的关闭标签
    const extraCloseTags = [
        /<\/div>\s*<\/div>\s*<\/div>\s*<\/div>/g,
        /<\/td>\s*<\/td>/g,
        /<\/button>\s*<\/button>/g
    ];
    
    extraCloseTags.forEach(pattern => {
        const originalContent = content;
        content = content.replace(pattern, (match) => {
            const tags = match.match(/<\/\w+>/g);
            if (tags && tags.length > 2) {
                console.log(`🔧 移除多余的关闭标签: ${match.trim()}`);
                modified = true;
                return tags.slice(0, 2).join('\n\t\t');
            }
            return match;
        });
    });
    
    // 5. 确保所有的 {#if} 都有对应的 {/if}
    const ifCount = (content.match(/\{#if/g) || []).length;
    const endIfCount = (content.match(/\{\/if\}/g) || []).length;
    
    if (ifCount > endIfCount) {
        const missing = ifCount - endIfCount;
        console.log(`🔧 添加 ${missing} 个缺失的 {/if}`);
        const lastIfIndex = content.lastIndexOf('{/if}');
        if (lastIfIndex > -1) {
            for (let i = 0; i < missing; i++) {
                content = content.slice(0, lastIfIndex) + '\n{/if}' + content.slice(lastIfIndex);
            }
            modified = true;
        }
    }
    
    // 6. 清理文件末尾的多余空行和标签
    content = content.replace(/\n\s*\n\s*\n/g, '\n\n');
    content = content.replace(/\s+$/, '\n');
    
    if (modified) {
        fs.writeFileSync(filePath, content);
        console.log('✅ 文件结构已修复');
        return true;
    } else {
        console.log('✅ 文件无需修复');
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
    console.log('🔧 开始修复所有结构问题...\n');
    
    let allFixed = true;
    for (const file of files) {
        const fixed = fixAllStructure(file);
        if (!fixed) allFixed = false;
    }
    
    if (allFixed) {
        console.log('\n🎉 所有文件结构修复完成！');
    } else {
        console.log('\n⚠️ 部分文件需要手动修复');
    }
}

if (require.main === module) {
    main();
}
