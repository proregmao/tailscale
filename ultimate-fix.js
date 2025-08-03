#!/usr/bin/env node

const fs = require('fs');

// 需要修复的文件
const files = [
    'headscale-ui/src/routes/routes.html/+page.svelte',
    'headscale-ui/src/routes/ssh.html/+page.svelte', 
    'headscale-ui/src/routes/serve.html/+page.svelte'
];

function ultimateFix(filePath) {
    console.log(`\n🔧 终极修复文件: ${filePath}`);
    
    if (!fs.existsSync(filePath)) {
        console.log(`❌ 文件不存在: ${filePath}`);
        return false;
    }
    
    let content = fs.readFileSync(filePath, 'utf8');
    let lines = content.split('\n');
    let modified = false;
    
    // 1. 确保第8行的主容器在文件末尾有对应的关闭标签
    let hasMainContainer = false;
    let mainContainerLine = -1;
    
    for (let i = 0; i < lines.length; i++) {
        if (lines[i].includes('class="bg-white min-h-screen"') && lines[i].includes('<div')) {
            hasMainContainer = true;
            mainContainerLine = i + 1;
            break;
        }
    }
    
    // 2. 找到最后的 {/if} 位置
    let lastIfIndex = -1;
    for (let i = lines.length - 1; i >= 0; i--) {
        if (lines[i].trim() === '{/if}') {
            lastIfIndex = i;
            break;
        }
    }
    
    if (hasMainContainer && lastIfIndex > 0) {
        // 在 {/if} 之前添加主容器的关闭标签
        const beforeIf = lines[lastIfIndex - 1];
        if (!beforeIf.includes('</div>') || beforeIf.trim() !== '</div>') {
            lines.splice(lastIfIndex, 0, '\t</div>');
            modified = true;
            console.log(`  ✓ 添加主容器关闭标签在第${lastIfIndex + 1}行`);
        }
    }
    
    // 3. 删除多余的 </div> 标签
    const extraDivPattern = /^\s*<\/div>\s*$/;
    let consecutiveDivs = 0;
    
    for (let i = lines.length - 1; i >= 0; i--) {
        if (extraDivPattern.test(lines[i])) {
            consecutiveDivs++;
            if (consecutiveDivs > 2) { // 保留最多2个连续的 </div>
                lines.splice(i, 1);
                modified = true;
                console.log(`  ✓ 删除第${i + 1}行的多余 </div>`);
            }
        } else {
            consecutiveDivs = 0;
        }
    }
    
    // 4. 删除无效标签
    const invalidTags = ['</svelte>', '<svelte>', '</script>', '<script>'];
    for (let i = lines.length - 1; i >= 0; i--) {
        const line = lines[i].trim();
        if (invalidTags.some(tag => line === tag)) {
            lines.splice(i, 1);
            modified = true;
            console.log(`  ✓ 删除第${i + 1}行的无效标签: ${line}`);
        }
    }
    
    // 5. 删除多余的 {/if}
    let ifCount = 0;
    for (let i = lines.length - 1; i >= 0; i--) {
        if (lines[i].trim() === '{/if}') {
            ifCount++;
            if (ifCount > 1) {
                lines.splice(i, 1);
                modified = true;
                console.log(`  ✓ 删除第${i + 1}行的多余 {/if}`);
            }
        }
    }
    
    // 6. 确保文件以正确的结构结束
    const lastLines = lines.slice(-5).map(l => l.trim()).filter(l => l);
    const expectedEnd = ['{/if}'];
    
    // 清理文件末尾的空行和多余内容
    while (lines.length > 0 && lines[lines.length - 1].trim() === '') {
        lines.pop();
        modified = true;
    }
    
    // 确保以 {/if} 结束
    if (lines.length > 0 && lines[lines.length - 1].trim() !== '{/if}') {
        // 查找最后一个 {/if}
        let lastIfFound = false;
        for (let i = lines.length - 1; i >= 0; i--) {
            if (lines[i].trim() === '{/if}') {
                // 删除 {/if} 之后的所有内容
                lines = lines.slice(0, i + 1);
                lastIfFound = true;
                modified = true;
                console.log(`  ✓ 清理 {/if} 之后的多余内容`);
                break;
            }
        }
    }
    
    if (modified) {
        fs.writeFileSync(filePath, lines.join('\n') + '\n');
        console.log('✅ 文件已修复');
        return true;
    } else {
        console.log('✅ 文件无需修复');
        return true;
    }
}

async function main() {
    console.log('🔧 开始终极修复 Svelte 文件...\n');
    
    let allFixed = true;
    for (const file of files) {
        const fixed = ultimateFix(file);
        if (!fixed) allFixed = false;
    }
    
    if (allFixed) {
        console.log('\n🎉 所有文件终极修复完成！');
    } else {
        console.log('\n⚠️ 部分文件需要手动修复');
    }
}

if (require.main === module) {
    main();
}
