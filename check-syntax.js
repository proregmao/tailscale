#!/usr/bin/env node

const fs = require('fs');
const { exec } = require('child_process');

// 需要检查的文件
const files = [
    'headscale-ui/src/routes/routes.html/+page.svelte',
    'headscale-ui/src/routes/ssh.html/+page.svelte',
    'headscale-ui/src/routes/serve.html/+page.svelte'
];

function checkSvelteFile(filePath) {
    console.log(`\n🔍 检查文件: ${filePath}`);
    
    if (!fs.existsSync(filePath)) {
        console.log(`❌ 文件不存在: ${filePath}`);
        return;
    }
    
    const content = fs.readFileSync(filePath, 'utf8');
    const lines = content.split('\n');
    
    // 检查基本结构
    const openDivs = (content.match(/<div[^>]*>/g) || []).length;
    const closeDivs = (content.match(/<\/div>/g) || []).length;
    const openIfs = (content.match(/\{#if/g) || []).length;
    const closeIfs = (content.match(/\{\/if\}/g) || []).length;
    
    console.log(`📊 结构统计:`);
    console.log(`   <div>: ${openDivs} 个开放, ${closeDivs} 个关闭 ${openDivs === closeDivs ? '✅' : '❌'}`);
    console.log(`   {#if}: ${openIfs} 个开放, ${closeIfs} 个关闭 ${openIfs === closeIfs ? '✅' : '❌'}`);
    
    // 检查常见问题
    const issues = [];
    
    // 检查未闭合的标签
    if (openDivs !== closeDivs) {
        issues.push(`div标签不匹配: ${openDivs} 开放, ${closeDivs} 关闭`);
    }
    
    if (openIfs !== closeIfs) {
        issues.push(`if块不匹配: ${openIfs} 开放, ${closeIfs} 关闭`);
    }
    
    // 检查常见的语法错误
    lines.forEach((line, index) => {
        const lineNum = index + 1;
        
        // 检查孤立的结束标签
        if (line.trim() === '</div>' && lines[index - 1] && lines[index - 1].trim() === '</div>') {
            issues.push(`第${lineNum}行: 可能有多余的 </div>`);
        }
        
        // 检查孤立的if结束
        if (line.trim() === '{/if}' && lines[index - 1] && lines[index - 1].trim() === '{/if}') {
            issues.push(`第${lineNum}行: 可能有多余的 {/if}`);
        }
    });
    
    if (issues.length === 0) {
        console.log('✅ 未发现明显的结构问题');
    } else {
        console.log('❌ 发现问题:');
        issues.forEach(issue => console.log(`   - ${issue}`));
    }
}

async function checkCompilation(file) {
    return new Promise((resolve) => {
        const cmd = `cd headscale-ui && npx svelte-check --tsconfig ./jsconfig.json --threshold error --output human ${file}`;
        exec(cmd, (error, stdout, stderr) => {
            if (error || stderr) {
                console.log(`\n❌ ${file} 编译错误:`);
                console.log(stderr || error.message);
            } else {
                console.log(`\n✅ ${file} 编译正常`);
            }
            resolve();
        });
    });
}

async function main() {
    console.log('🔧 检查 Svelte 文件语法...');
    
    for (const file of files) {
        checkSvelteFile(file);
        await checkCompilation(file);
    }
}

if (require.main === module) {
    main();
}
