#!/usr/bin/env node

/**
 * 设备注册功能测试
 * 测试密钥格式修复后的设备注册功能
 */

const http = require('http');

// 测试配置
const API_BASE = 'http://localhost:8081';
const API_KEY = 'test-api-key-12345';

// 生成测试用的NodeKey（64位十六进制）
function generateTestNodeKey() {
    const chars = '0123456789abcdef';
    let result = '';
    for (let i = 0; i < 64; i++) {
        result += chars.charAt(Math.floor(Math.random() * chars.length));
    }
    return result;
}

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

// 测试设备注册
async function testDeviceRegistration() {
    console.log('🔧 测试设备注册功能...\n');
    
    try {
        // 1. 生成测试密钥
        const rawKey = generateTestNodeKey();
        const nodeKey = `nodekey:${rawKey}`;
        
        console.log(`📝 生成测试密钥:`);
        console.log(`   原始密钥: ${rawKey}`);
        console.log(`   格式化密钥: ${nodeKey}\n`);
        
        // 2. 构建注册请求
        const registerRequest = {
            NodeKey: nodeKey,
            Hostinfo: {
                Hostname: `test-device-${Date.now()}`,
                OS: "linux"
            },
            User: "1"
        };
        
        console.log('📤 发送注册请求...');
        console.log(`   请求体: ${JSON.stringify(registerRequest, null, 2)}\n`);
        
        // 3. 发送注册请求
        const response = await makeRequest(`${API_BASE}/machine/register`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${API_KEY}`
            },
            body: JSON.stringify(registerRequest)
        });
        
        console.log(`📥 注册响应:`);
        console.log(`   状态码: ${response.status}`);
        console.log(`   响应体: ${JSON.stringify(response.data, null, 2)}\n`);
        
        // 4. 验证结果
        if (response.status === 200) {
            console.log('✅ 设备注册成功!');
            
            // 验证响应格式
            if (response.data.User && response.data.Login) {
                console.log('✅ 响应格式正确');
                console.log(`   用户ID: ${response.data.User.ID}`);
                console.log(`   用户名: ${response.data.User.DisplayName}`);
                console.log(`   机器授权: ${response.data.MachineAuthorized}`);
            } else {
                console.log('⚠️  响应格式异常，但注册成功');
            }
            
            return true;
        } else {
            console.log(`❌ 设备注册失败: HTTP ${response.status}`);
            if (response.data.error) {
                console.log(`   错误信息: ${response.data.error}`);
            }
            return false;
        }
        
    } catch (error) {
        console.log(`❌ 设备注册测试异常: ${error.message}`);
        return false;
    }
}

// 测试密钥格式验证
async function testKeyFormatValidation() {
    console.log('\n🔑 测试密钥格式验证...\n');
    
    const testCases = [
        {
            name: '正确格式的密钥',
            key: `nodekey:${generateTestNodeKey()}`,
            shouldSucceed: true
        },
        {
            name: '无前缀的密钥',
            key: generateTestNodeKey(),
            shouldSucceed: false
        },
        {
            name: '错误前缀的密钥',
            key: `wrongkey:${generateTestNodeKey()}`,
            shouldSucceed: false
        },
        {
            name: '空密钥',
            key: '',
            shouldSucceed: false
        }
    ];
    
    let passCount = 0;
    
    for (const testCase of testCases) {
        console.log(`📝 测试: ${testCase.name}`);
        console.log(`   密钥: ${testCase.key.substring(0, 20)}...`);
        
        try {
            const registerRequest = {
                NodeKey: testCase.key,
                Hostinfo: {
                    Hostname: `test-${Date.now()}`,
                    OS: "linux"
                },
                User: "1"
            };
            
            const response = await makeRequest(`${API_BASE}/machine/register`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${API_KEY}`
                },
                body: JSON.stringify(registerRequest)
            });
            
            const success = response.status === 200;
            const expected = testCase.shouldSucceed;
            
            if (success === expected) {
                console.log(`   ✅ 结果符合预期: ${success ? '成功' : '失败'}`);
                passCount++;
            } else {
                console.log(`   ❌ 结果不符合预期: 期望${expected ? '成功' : '失败'}，实际${success ? '成功' : '失败'}`);
                if (response.data.error) {
                    console.log(`   错误: ${response.data.error}`);
                }
            }
            
        } catch (error) {
            console.log(`   ❌ 测试异常: ${error.message}`);
        }
        
        console.log('');
    }
    
    console.log(`📊 密钥格式验证结果: ${passCount}/${testCases.length} 通过\n`);
    return passCount === testCases.length;
}

// 主测试函数
async function main() {
    console.log('🚀 设备注册功能测试开始...\n');
    console.log('=' .repeat(50));
    
    const results = [];
    
    // 测试设备注册
    const registrationResult = await testDeviceRegistration();
    results.push({ name: '设备注册', passed: registrationResult });
    
    // 测试密钥格式验证
    const keyFormatResult = await testKeyFormatValidation();
    results.push({ name: '密钥格式验证', passed: keyFormatResult });
    
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
        console.log('\n🎉 所有测试通过！设备注册功能正常工作！');
    } else {
        console.log('\n⚠️  部分测试失败，请检查相关功能。');
    }
}

// 运行测试
main().catch(console.error);
