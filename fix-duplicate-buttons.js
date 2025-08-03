#!/usr/bin/env node

const fs = require('fs');

function fixDuplicateButtons(filePath) {
    console.log(`\n🔧 修复重复的 button 标签: ${filePath}`);
    
    if (!fs.existsSync(filePath)) {
        console.log(`❌ 文件不存在: ${filePath}`);
        return false;
    }
    
    let content = fs.readFileSync(filePath, 'utf8');
    let modified = false;
    
    // 修复重复的 </button> 标签
    const duplicateButtonPattern = /<\/button>\s*<\/button>/g;
    const originalContent = content;
    content = content.replace(duplicateButtonPattern, '</button>');
    
    if (content !== originalContent) {
        const matches = originalContent.match(duplicateButtonPattern) || [];
        console.log(`🔧 修复了 ${matches.length} 个重复的 </button> 标签`);
        modified = true;
    }
    
    // 修复重复的 </select> 标签
    const duplicateSelectPattern = /<\/select>\s*<\/select>/g;
    const beforeSelect = content;
    content = content.replace(duplicateSelectPattern, '</select>');
    
    if (content !== beforeSelect) {
        const matches = beforeSelect.match(duplicateSelectPattern) || [];
        console.log(`🔧 修复了 ${matches.length} 个重复的 </select> 标签`);
        modified = true;
    }
    
    // 修复重复的 </div> 标签（最多保留2个连续的）
    const duplicateDivPattern = /(<\/div>\s*){3,}/g;
    const beforeDiv = content;
    content = content.replace(duplicateDivPattern, '</div>\n\t\t</div>');
    
    if (content !== beforeDiv) {
        console.log(`🔧 修复了多余的连续 </div> 标签`);
        modified = true;
    }
    
    // 修复重复的 </td> 标签
    const duplicateTdPattern = /<\/td>\s*<\/td>/g;
    const beforeTd = content;
    content = content.replace(duplicateTdPattern, '</td>');
    
    if (content !== beforeTd) {
        const matches = beforeTd.match(duplicateTdPattern) || [];
        console.log(`🔧 修复了 ${matches.length} 个重复的 </td> 标签`);
        modified = true;
    }
    
    // 修复重复的 </textarea> 标签
    const duplicateTextareaPattern = /<\/textarea>\s*<\/textarea>/g;
    const beforeTextarea = content;
    content = content.replace(duplicateTextareaPattern, '</textarea>');
    
    if (content !== beforeTextarea) {
        const matches = beforeTextarea.match(duplicateTextareaPattern) || [];
        console.log(`🔧 修复了 ${matches.length} 个重复的 </textarea> 标签`);
        modified = true;
    }
    
    if (modified) {
        fs.writeFileSync(filePath, content);
        console.log('✅ 重复标签已修复');
        return true;
    } else {
        console.log('✅ 无重复标签需要修复');
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
    console.log('🔧 开始修复重复标签...\n');
    
    let allFixed = true;
    for (const file of files) {
        const fixed = fixDuplicateButtons(file);
        if (!fixed) allFixed = false;
    }
    
    if (allFixed) {
        console.log('\n🎉 所有重复标签修复完成！');
    } else {
        console.log('\n⚠️ 部分文件需要手动修复');
    }
}

if (require.main === module) {
    main();
}
