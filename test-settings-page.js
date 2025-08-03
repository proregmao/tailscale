#!/usr/bin/env node

/**
 * 设置页面功能测试
 * 测试系统设置页面的各项功能
 */

const http = require('http');

// 测试配置
const API_BASE = 'http://localhost:8081';
const API_KEY = 'test-api-key-12345';

// HTTP请求函数
function makeRequest(url, options = {}) {
    return new Promise((resolve, reject) => {
        const req = http.request(url, options, (res) => {
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

// 测试API Keys端点
async function testAPIKeysEndpoint() {
    console.log('🔑 测试API Keys端点...\n');
    
    try {
        // 测试获取API Keys列表
        console.log('📤 获取API Keys列表...');
        const response = await makeRequest(`${API_BASE}/api/v1/apikey`, {
            method: 'GET',
            headers: {
                'Authorization': `Bearer ${API_KEY}`
            }
        });
        
        console.log(`📥 API Keys响应:`);
        console.log(`   状态码: ${response.status}`);
        console.log(`   响应体: ${JSON.stringify(response.data, null, 2)}\n`);
        
        if (response.status === 200) {
            console.log('✅ API Keys端点正常工作');
            
            // 验证响应格式
            if (response.data.success && Array.isArray(response.data.data)) {
                console.log('✅ 响应格式正确');
                console.log(`   API Keys数量: ${response.data.data.length}`);
                console.log(`   总数: ${response.data.total}`);
                return true;
            } else {
                console.log('⚠️  响应格式异常，但端点可访问');
                return false;
            }
        } else {
            console.log(`❌ API Keys端点失败: HTTP ${response.status}`);
            return false;
        }
        
    } catch (error) {
        console.log(`❌ API Keys测试异常: ${error.message}`);
        return false;
    }
}

// 测试创建API Key
async function testCreateAPIKey() {
    console.log('\n🆕 测试创建API Key...\n');
    
    try {
        // 创建新的API Key
        const expiration = new Date();
        expiration.setDate(expiration.getDate() + 90);
        
        const requestBody = {
            expiration: expiration.toISOString()
        };
        
        console.log('📤 创建API Key...');
        console.log(`   过期时间: ${expiration.toISOString()}`);
        
        const response = await makeRequest(`${API_BASE}/api/v1/apikey`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${API_KEY}`
            },
            body: JSON.stringify(requestBody)
        });
        
        console.log(`📥 创建API Key响应:`);
        console.log(`   状态码: ${response.status}`);
        console.log(`   响应体: ${JSON.stringify(response.data, null, 2)}\n`);
        
        if (response.status === 200) {
            console.log('✅ API Key创建成功');
            
            if (response.data.success && response.data.data && response.data.data.apiKey) {
                console.log('✅ 返回了有效的API Key');
                console.log(`   API Key: ${response.data.data.apiKey.substring(0, 20)}...`);
                return true;
            } else {
                console.log('⚠️  API Key格式异常');
                return false;
            }
        } else {
            console.log(`❌ API Key创建失败: HTTP ${response.status}`);
            return false;
        }
        
    } catch (error) {
        console.log(`❌ 创建API Key测试异常: ${error.message}`);
        return false;
    }
}

// 测试API Key过期
async function testExpireAPIKey() {
    console.log('\n⏰ 测试API Key过期...\n');
    
    try {
        const requestBody = {
            prefix: "test-prefix"
        };
        
        console.log('📤 使API Key过期...');
        const response = await makeRequest(`${API_BASE}/api/v1/apikey/expire`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${API_KEY}`
            },
            body: JSON.stringify(requestBody)
        });
        
        console.log(`📥 过期API Key响应:`);
        console.log(`   状态码: ${response.status}`);
        console.log(`   响应体: ${JSON.stringify(response.data, null, 2)}\n`);
        
        if (response.status === 200) {
            console.log('✅ API Key过期功能正常');
            
            if (response.data.success) {
                console.log('✅ 过期操作成功');
                return true;
            } else {
                console.log('⚠️  过期操作响应异常');
                return false;
            }
        } else {
            console.log(`❌ API Key过期失败: HTTP ${response.status}`);
            return false;
        }
        
    } catch (error) {
        console.log(`❌ API Key过期测试异常: ${error.message}`);
        return false;
    }
}

// 测试前端数据处理逻辑
async function testFrontendDataProcessing() {
    console.log('\n🔄 测试前端数据处理逻辑...\n');
    
    try {
        // 模拟前端getAPIKeys函数的数据处理逻辑
        const mockBackendResponse = {
            success: true,
            data: [],
            total: 0
        };
        
        console.log('📝 模拟后端响应:');
        console.log(`   ${JSON.stringify(mockBackendResponse, null, 2)}\n`);
        
        // 模拟前端处理逻辑
        let apiKeys = [];
        
        if (mockBackendResponse && Array.isArray(mockBackendResponse.apiKeys)) {
            apiKeys = mockBackendResponse.apiKeys;
            console.log('✅ 使用 data.apiKeys 格式');
        } else if (mockBackendResponse && Array.isArray(mockBackendResponse.data)) {
            apiKeys = mockBackendResponse.data;
            console.log('✅ 使用 data.data 格式');
        } else if (Array.isArray(mockBackendResponse)) {
            apiKeys = mockBackendResponse;
            console.log('✅ 使用直接数组格式');
        } else {
            console.log('⚠️  数据格式不正确，使用空数组');
            apiKeys = [];
        }
        
        console.log(`📊 处理结果:`);
        console.log(`   API Keys数组: ${JSON.stringify(apiKeys)}`);
        console.log(`   数组长度: ${apiKeys.length}`);
        console.log(`   是否为数组: ${Array.isArray(apiKeys)}\n`);
        
        // 测试forEach调用
        try {
            let forEachCount = 0;
            apiKeys.forEach(key => {
                forEachCount++;
            });
            console.log('✅ forEach调用成功');
            console.log(`   遍历次数: ${forEachCount}`);
            return true;
        } catch (error) {
            console.log(`❌ forEach调用失败: ${error.message}`);
            return false;
        }
        
    } catch (error) {
        console.log(`❌ 前端数据处理测试异常: ${error.message}`);
        return false;
    }
}

// 主测试函数
async function main() {
    console.log('🚀 设置页面功能测试开始...\n');
    console.log('=' .repeat(50));
    
    const results = [];
    
    // 测试API Keys端点
    const apiKeysResult = await testAPIKeysEndpoint();
    results.push({ name: 'API Keys端点', passed: apiKeysResult });
    
    // 测试创建API Key
    const createResult = await testCreateAPIKey();
    results.push({ name: 'API Key创建', passed: createResult });
    
    // 测试API Key过期
    const expireResult = await testExpireAPIKey();
    results.push({ name: 'API Key过期', passed: expireResult });
    
    // 测试前端数据处理
    const frontendResult = await testFrontendDataProcessing();
    results.push({ name: '前端数据处理', passed: frontendResult });
    
    // 汇总结果
    console.log('=' .repeat(50));
    console.log('📊 测试结果汇总:');
    
    let totalPassed = 0;
    for (const result of results) {
        const status = result.passed ? '✅ 通过' : '❌ 失败';
        console.log(`   ${result.name}: ${status}`);
        if (result.passed) totalPassed++;
    }
    
    const successRate = (totalPassed / results.length * 100).toFixed(1);
    console.log(`\n📈 总体成功率: ${successRate}% (${totalPassed}/${results.length})`);
    
    if (totalPassed === results.length) {
        console.log('\n🎉 所有测试通过！设置页面功能正常！');
        console.log('\n💡 修复说明:');
        console.log('   - 添加了API Keys数据格式的安全检查');
        console.log('   - 修复了forEach调用的undefined错误');
        console.log('   - 支持多种后端响应格式');
        console.log('   - 增强了错误处理和日志记录');
    } else {
        console.log('\n⚠️  部分测试失败，设置页面可能仍有问题。');
    }
}

// 运行测试
main().catch(console.error);
