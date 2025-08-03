#!/usr/bin/env node

const { exec } = require('child_process');
const fs = require('fs');

// 需要调试的页面
const pages = [
    'routes.html',
    'ssh.html', 
    'serve.html'
];

async function debugPage(page) {
    return new Promise((resolve) => {
        console.log(`\n🔍 调试页面: ${page}`);
        
        const url = `http://localhost:5173/${page}`;
        
        // 1. 检查HTTP状态码
        exec(`curl -s -o /dev/null -w "%{http_code}" ${url}`, (error, stdout, stderr) => {
            const statusCode = stdout.trim();
            console.log(`   HTTP状态: ${statusCode}`);
            
            if (statusCode === '500') {
                // 2. 获取错误页面内容
                exec(`curl -s ${url} | head -50`, (err, out, serr) => {
                    if (out.includes('Error') || out.includes('error')) {
                        console.log(`   错误内容: ${out.split('\n').slice(0, 5).join(' ')}`);
                    }
                    
                    // 3. 检查前端日志中的相关错误
                    exec(`tail -200 logs/frontend.log | grep -A 5 -B 5 "${page}"`, (e, o, s) => {
                        if (o && o.trim()) {
                            console.log(`   前端日志错误:`);
                            console.log(`   ${o.split('\n').slice(-10).join('\n   ')}`);
                        }
                        
                        // 4. 尝试编译检查
                        const filePath = `headscale-ui/src/routes/${page}/+page.svelte`;
                        if (fs.existsSync(filePath)) {
                            exec(`cd headscale-ui && npx svelte-check --tsconfig ./jsconfig.json --threshold error ${filePath} 2>&1`, (ce, co, cs) => {
                                if (cs || co.includes('Error')) {
                                    console.log(`   编译错误: ${cs || co}`);
                                } else {
                                    console.log(`   编译检查: 通过`);
                                }
                                resolve();
                            });
                        } else {
                            console.log(`   文件不存在: ${filePath}`);
                            resolve();
                        }
                    });
                });
            } else {
                console.log(`   页面正常`);
                resolve();
            }
        });
    });
}

async function checkSvelteKitErrors() {
    console.log('\n🔍 检查 SvelteKit 构建错误...');
    
    return new Promise((resolve) => {
        exec('cd headscale-ui && npm run build 2>&1 | tail -50', (error, stdout, stderr) => {
            if (error || stderr) {
                console.log('构建错误:');
                console.log(stderr || error.message);
            } else if (stdout.includes('Error') || stdout.includes('error')) {
                console.log('构建警告/错误:');
                console.log(stdout);
            } else {
                console.log('构建检查: 通过');
            }
            resolve();
        });
    });
}

async function main() {
    console.log('🔍 开始深度调试页面错误...');
    
    // 1. 调试每个失败的页面
    for (const page of pages) {
        await debugPage(page);
        await new Promise(resolve => setTimeout(resolve, 1000)); // 等待1秒
    }
    
    // 2. 检查 SvelteKit 构建错误
    await checkSvelteKitErrors();
    
    console.log('\n📊 调试完成');
}

if (require.main === module) {
    main();
}
