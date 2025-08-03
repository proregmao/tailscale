#!/usr/bin/env node

const fs = require('fs');
const { exec } = require('child_process');

// 需要测试的页面
const pages = [
    'routes.html',
    'ssh.html', 
    'serve.html'
];

async function testPage(page) {
    return new Promise((resolve) => {
        const url = `http://localhost:5173/${page}`;
        exec(`curl -s -o /dev/null -w "%{http_code}" ${url}`, (error, stdout, stderr) => {
            const statusCode = stdout.trim();
            console.log(`📄 ${page}: HTTP ${statusCode} ${statusCode === '200' ? '✅' : '❌'}`);
            
            if (statusCode !== '200') {
                // 获取详细错误信息
                exec(`curl -s ${url} | head -20`, (err, out, serr) => {
                    if (out.includes('Error') || out.includes('error')) {
                        console.log(`   错误信息: ${out.split('\n')[0]}`);
                    }
                });
            }
            resolve(statusCode === '200');
        });
    });
}

async function main() {
    console.log('🧪 测试页面加载状态...\n');
    
    let passCount = 0;
    for (const page of pages) {
        const success = await testPage(page);
        if (success) passCount++;
        await new Promise(resolve => setTimeout(resolve, 1000)); // 等待1秒
    }
    
    console.log(`\n📊 测试结果: ${passCount}/${pages.length} 页面正常`);
    console.log(`成功率: ${Math.round(passCount / pages.length * 100)}%`);
    
    if (passCount < pages.length) {
        console.log('\n🔍 检查前端日志中的错误...');
        exec('tail -50 logs/frontend.log | grep -i error', (error, stdout, stderr) => {
            if (stdout) {
                console.log('前端错误:', stdout);
            } else {
                console.log('前端日志中未发现明显错误');
            }
        });
    }
}

if (require.main === module) {
    main();
}
