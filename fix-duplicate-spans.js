#!/usr/bin/env node

const fs = require('fs');

function fixDuplicateSpans(filePath) {
    console.log(`\n🔧 修复重复的 span 标签: ${filePath}`);
    
    if (!fs.existsSync(filePath)) {
        console.log(`❌ 文件不存在: ${filePath}`);
        return false;
    }
    
    let content = fs.readFileSync(filePath, 'utf8');
    let modified = false;
    
    // 修复重复的 </span> 标签
    const duplicateSpanPattern = /<\/span>\s*<\/span>/g;
    const originalContent = content;
    content = content.replace(duplicateSpanPattern, '</span>');
    
    if (content !== originalContent) {
        const matches = originalContent.match(duplicateSpanPattern) || [];
        console.log(`🔧 修复了 ${matches.length} 个重复的 </span> 标签`);
        modified = true;
    }
    
    // 修复其他可能的重复标签
    const otherDuplicates = [
        { pattern: /<\/div>\s*<\/div>\s*<\/div>/g, name: 'div' },
        { pattern: /<\/td>\s*<\/td>/g, name: 'td' },
        { pattern: /<\/th>\s*<\/th>/g, name: 'th' },
        { pattern: /<\/tr>\s*<\/tr>/g, name: 'tr' }
    ];
    
    otherDuplicates.forEach(({ pattern, name }) => {
        const before = content;
        content = content.replace(pattern, `</${name}>`);
        if (content !== before) {
            const matches = before.match(pattern) || [];
            console.log(`🔧 修复了 ${matches.length} 个重复的 </${name}> 标签`);
            modified = true;
        }
    });
    
    if (modified) {
        fs.writeFileSync(filePath, content);
        console.log('✅ 重复 span 标签已修复');
        return true;
    } else {
        console.log('✅ 无重复 span 标签需要修复');
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
    console.log('🔧 开始修复重复 span 标签...\n');
    
    let allFixed = true;
    for (const file of files) {
        const fixed = fixDuplicateSpans(file);
        if (!fixed) allFixed = false;
    }
    
    if (allFixed) {
        console.log('\n🎉 所有重复 span 标签修复完成！');
    } else {
        console.log('\n⚠️ 部分文件需要手动修复');
    }
}

if (require.main === module) {
    main();
}
