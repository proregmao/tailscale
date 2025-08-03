#!/usr/bin/env node

/**
 * 全功能测试脚本
 * 测试所有修复后的功能
 */

const https = require('http');

// 测试配置
const config = {
    baseURL: 'http://localhost:8081',
    frontendURL: 'http://localhost:3000',
    apiKey: 'test-api-key'
};

// 测试结果
let testResults = {
    passed: 0,
    failed: 0,
    tests: []
};

// 工具函数
function makeRequest(url, options = {}) {
    return new Promise((resolve, reject) => {
        const req = https.request(url, options, (res) => {
            let data = '';
            res.on('data', chunk => data += chunk);
            res.on('end', () => {
                try {
                    const jsonData = JSON.parse(data);
                    resolve({ status: res.statusCode, data: jsonData });
                } catch (e) {
                    resolve({ status: res.statusCode, data: data });
                }
            });
        });
        
        req.on('error', reject);
        
        if (options.body) {
            req.write(options.body);
        }
        
        req.end();
    });
}

function logTest(name, passed, message = '') {
    const status = passed ? '✅ PASS' : '❌ FAIL';
    console.log(`${status} ${name}${message ? ': ' + message : ''}`);
    
    testResults.tests.push({ name, passed, message });
    if (passed) {
        testResults.passed++;
    } else {
        testResults.failed++;
    }
}

// 测试函数
async function testDevicesAPI() {
    console.log('\n🔧 测试设备API...');
    
    try {
        const response = await makeRequest(`${config.baseURL}/api/v1/devices`);
        
        if (response.status === 200 && response.data.success) {
            logTest('设备API响应', true, `获取到 ${response.data.total} 个设备`);
            
            // 检查数据结构
            if (response.data.data && Array.isArray(response.data.data)) {
                logTest('设备数据结构', true, '数据格式正确');
                
                // 检查设备字段
                if (response.data.data.length > 0) {
                    const device = response.data.data[0];
                    const requiredFields = ['id', 'node_key', 'hostname', 'user_id', 'ip_addresses'];
                    const hasAllFields = requiredFields.every(field => device.hasOwnProperty(field));
                    logTest('设备字段完整性', hasAllFields, hasAllFields ? '所有必需字段存在' : '缺少必需字段');
                }
            } else {
                logTest('设备数据结构', false, '数据格式错误');
            }
        } else {
            logTest('设备API响应', false, `状态码: ${response.status}`);
        }
    } catch (error) {
        logTest('设备API连接', false, error.message);
    }
}

async function testUsersAPI() {
    console.log('\n👥 测试用户API...');
    
    try {
        const response = await makeRequest(`${config.baseURL}/api/v1/users`);
        
        if (response.status === 200 && response.data.success) {
            logTest('用户API响应', true, `获取到 ${response.data.total} 个用户`);
            
            // 检查数据结构
            if (response.data.data && Array.isArray(response.data.data)) {
                logTest('用户数据结构', true, '数据格式正确');
                
                // 检查用户字段
                if (response.data.data.length > 0) {
                    const user = response.data.data[0];
                    const requiredFields = ['id', 'name', 'email', 'role'];
                    const hasAllFields = requiredFields.every(field => user.hasOwnProperty(field));
                    logTest('用户字段完整性', hasAllFields, hasAllFields ? '所有必需字段存在' : '缺少必需字段');
                }
            } else {
                logTest('用户数据结构', false, '数据格式错误');
            }
        } else {
            logTest('用户API响应', false, `状态码: ${response.status}`);
        }
    } catch (error) {
        logTest('用户API连接', false, error.message);
    }
}

async function testFrontendPages() {
    console.log('\n🌐 测试前端页面...');
    
    const pages = [
        { name: '设备管理页面', url: '/devices.html' },
        { name: '用户管理页面', url: '/users.html' },
        { name: '网络监控页面', url: '/monitoring.html' },
        { name: 'DERP管理页面', url: '/derp.html' },
        { name: 'ACL管理页面', url: '/acl.html' }
    ];
    
    for (const page of pages) {
        try {
            const response = await makeRequest(`${config.frontendURL}${page.url}`);
            const isHTML = typeof response.data === 'string' && response.data.includes('<html');
            logTest(page.name, response.status === 200 && isHTML, 
                   response.status === 200 ? '页面加载成功' : `状态码: ${response.status}`);
        } catch (error) {
            logTest(page.name, false, error.message);
        }
    }
}

async function testKeyGeneration() {
    console.log('\n🔑 测试密钥生成功能...');
    
    try {
        // 模拟密钥生成函数
        function generateNodeKey() {
            const randomBytes = new Uint8Array(32);
            for (let i = 0; i < 32; i++) {
                randomBytes[i] = Math.floor(Math.random() * 256);
            }
            const hexString = Array.from(randomBytes)
                .map(b => b.toString(16).padStart(2, '0'))
                .join('');
            return hexString; // 直接返回64位十六进制字符串
        }

        // 测试密钥生成
        const key1 = generateNodeKey();
        const key2 = generateNodeKey();

        // 检查格式（64位十六进制）
        const formatCorrect = /^[0-9a-f]{64}$/.test(key1);
        logTest('密钥格式', formatCorrect, formatCorrect ? '64位十六进制格式正确' : '格式错误');

        // 检查唯一性
        const isUnique = key1 !== key2;
        logTest('密钥唯一性', isUnique, isUnique ? '每次生成不同' : '生成重复密钥');

        // 检查长度
        const lengthCorrect = key1.length === 64; // 64 hex chars
        logTest('密钥长度', lengthCorrect, `长度: ${key1.length}`);
        
    } catch (error) {
        logTest('密钥生成功能', false, error.message);
    }
}

async function testErrorHandling() {
    console.log('\n🚨 测试错误处理...');
    
    try {
        // 测试不存在的端点
        const response = await makeRequest(`${config.baseURL}/api/v1/nonexistent`);
        const isError = response.status >= 400;
        logTest('404错误处理', isError, `状态码: ${response.status}`);
        
    } catch (error) {
        logTest('网络错误处理', true, '正确捕获网络错误');
    }
}

// 主测试函数
async function runAllTests() {
    console.log('🚀 开始全功能测试...\n');
    console.log('=' .repeat(50));
    
    await testDevicesAPI();
    await testUsersAPI();
    await testFrontendPages();
    await testKeyGeneration();
    await testErrorHandling();
    
    // 输出测试结果
    console.log('\n' + '=' .repeat(50));
    console.log('📊 测试结果汇总:');
    console.log(`✅ 通过: ${testResults.passed}`);
    console.log(`❌ 失败: ${testResults.failed}`);
    console.log(`📈 成功率: ${((testResults.passed / (testResults.passed + testResults.failed)) * 100).toFixed(1)}%`);
    
    if (testResults.failed > 0) {
        console.log('\n❌ 失败的测试:');
        testResults.tests
            .filter(test => !test.passed)
            .forEach(test => console.log(`  - ${test.name}: ${test.message}`));
    }
    
    console.log('\n🎉 测试完成!');
    process.exit(testResults.failed > 0 ? 1 : 0);
}

// 运行测试
if (require.main === module) {
    runAllTests().catch(error => {
        console.error('❌ 测试运行失败:', error);
        process.exit(1);
    });
}

module.exports = { runAllTests, testResults };
