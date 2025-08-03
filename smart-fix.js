#!/usr/bin/env node

const fs = require('fs');

// 需要修复的文件
const files = [
    'headscale-ui/src/routes/routes.html/+page.svelte',
    'headscale-ui/src/routes/ssh.html/+page.svelte', 
    'headscale-ui/src/routes/serve.html/+page.svelte'
];

function smartFix(filePath) {
    console.log(`\n🧠 智能修复文件: ${filePath}`);
    
    if (!fs.existsSync(filePath)) {
        console.log(`❌ 文件不存在: ${filePath}`);
        return false;
    }
    
    let content = fs.readFileSync(filePath, 'utf8');
    let lines = content.split('\n');
    
    // 统计标签
    let openDivs = 0;
    let closeDivs = 0;
    let openIfs = 0;
    let closeIfs = 0;
    
    lines.forEach(line => {
        openDivs += (line.match(/<div[^>]*>/g) || []).length;
        closeDivs += (line.match(/<\/div>/g) || []).length;
        openIfs += (line.match(/\{#if/g) || []).length;
        closeIfs += (line.match(/\{\/if\}/g) || []).length;
    });
    
    console.log(`📊 当前统计:`);
    console.log(`   <div>: ${openDivs} 开放, ${closeDivs} 关闭`);
    console.log(`   {#if}: ${openIfs} 开放, ${closeIfs} 关闭`);
    
    let modified = false;
    
    // 找到最后的 {/if} 位置
    let lastIfIndex = -1;
    for (let i = lines.length - 1; i >= 0; i--) {
        if (lines[i].trim() === '{/if}') {
            lastIfIndex = i;
            break;
        }
    }
    
    if (lastIfIndex === -1) {
        console.log('❌ 未找到 {/if}，添加到文件末尾');
        lines.push('{/if}');
        lastIfIndex = lines.length - 1;
        modified = true;
    }
    
    // 在 {/if} 之前添加缺少的关闭标签
    const missingDivs = openDivs - closeDivs;
    const missingIfs = openIfs - closeIfs;
    
    if (missingDivs > 0) {
        console.log(`🔧 添加 ${missingDivs} 个缺少的 </div>`);
        for (let i = 0; i < missingDivs; i++) {
            lines.splice(lastIfIndex, 0, '\t</div>');
            lastIfIndex++;
            modified = true;
        }
    }
    
    if (missingIfs > 0) {
        console.log(`🔧 添加 ${missingIfs} 个缺少的 {/if}`);
        for (let i = 0; i < missingIfs; i++) {
            lines.splice(lastIfIndex, 0, '\t{/if}');
            lastIfIndex++;
            modified = true;
        }
    }
    
    // 删除多余的关闭标签
    if (missingDivs < 0) {
        console.log(`🔧 删除 ${Math.abs(missingDivs)} 个多余的 </div>`);
        let toRemove = Math.abs(missingDivs);
        for (let i = lines.length - 1; i >= 0 && toRemove > 0; i--) {
            if (lines[i].trim() === '</div>') {
                lines.splice(i, 1);
                toRemove--;
                modified = true;
            }
        }
    }
    
    if (missingIfs < 0) {
        console.log(`🔧 删除 ${Math.abs(missingIfs)} 个多余的 {/if}`);
        let toRemove = Math.abs(missingIfs);
        for (let i = lines.length - 1; i >= 0 && toRemove > 0; i--) {
            if (lines[i].trim() === '{/if}') {
                lines.splice(i, 1);
                toRemove--;
                modified = true;
            }
        }
    }
    
    // 确保文件以正确的结构结束
    // 清理文件末尾的空行
    while (lines.length > 0 && lines[lines.length - 1].trim() === '') {
        lines.pop();
        modified = true;
    }
    
    // 确保最后一行是 {/if}
    if (lines.length > 0 && lines[lines.length - 1].trim() !== '{/if}') {
        lines.push('{/if}');
        modified = true;
    }
    
    if (modified) {
        fs.writeFileSync(filePath, lines.join('\n') + '\n');
        console.log('✅ 文件已修复');
        
        // 重新统计验证
        const newContent = fs.readFileSync(filePath, 'utf8');
        const newLines = newContent.split('\n');
        let newOpenDivs = 0;
        let newCloseDivs = 0;
        let newOpenIfs = 0;
        let newCloseIfs = 0;
        
        newLines.forEach(line => {
            newOpenDivs += (line.match(/<div[^>]*>/g) || []).length;
            newCloseDivs += (line.match(/<\/div>/g) || []).length;
            newOpenIfs += (line.match(/\{#if/g) || []).length;
            newCloseIfs += (line.match(/\{\/if\}/g) || []).length;
        });
        
        console.log(`📊 修复后统计:`);
        console.log(`   <div>: ${newOpenDivs} 开放, ${newCloseDivs} 关闭 ${newOpenDivs === newCloseDivs ? '✅' : '❌'}`);
        console.log(`   {#if}: ${newOpenIfs} 开放, ${newCloseIfs} 关闭 ${newOpenIfs === newCloseIfs ? '✅' : '❌'}`);
        
        return true;
    } else {
        console.log('✅ 文件无需修复');
        return true;
    }
}

async function main() {
    console.log('🧠 开始智能修复 Svelte 文件...\n');
    
    let allFixed = true;
    for (const file of files) {
        const fixed = smartFix(file);
        if (!fixed) allFixed = false;
    }
    
    if (allFixed) {
        console.log('\n🎉 所有文件智能修复完成！');
    } else {
        console.log('\n⚠️ 部分文件需要手动修复');
    }
}

if (require.main === module) {
    main();
}
