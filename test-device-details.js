#!/usr/bin/env node

/**
 * 设备详情显示测试
 * 测试设备技术信息和网络详情的显示
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

// 测试设备详情数据
async function testDeviceDetails() {
    console.log('🔧 测试设备详情数据...\n');
    
    try {
        // 1. 获取设备列表
        console.log('📤 获取设备列表...');
        const response = await makeRequest(`${API_BASE}/api/v1/devices`, {
            method: 'GET',
            headers: {
                'Authorization': `Bearer ${API_KEY}`
            }
        });
        
        console.log(`📥 设备列表响应:`);
        console.log(`   状态码: ${response.status}`);
        console.log(`   设备数量: ${response.data.data?.length || 0}\n`);
        
        if (response.status !== 200 || !response.data.data) {
            console.log('❌ 获取设备列表失败');
            return false;
        }
        
        const devices = response.data.data;
        
        // 2. 分析每个设备的详情
        console.log('📊 设备详情分析:\n');
        
        let hasCompleteData = true;
        
        for (let i = 0; i < devices.length; i++) {
            const device = devices[i];
            console.log(`📱 设备 ${i + 1}: ${device.hostname || device.given_name || '未命名'}`);
            console.log(`   ID: ${device.id}`);
            
            // 检查技术信息
            console.log('   🔧 技术信息:');
            console.log(`      节点密钥: ${device.node_key ? '✅ 已设置' : '❌ 未设置'}`);
            console.log(`      机器密钥: ${device.machine_key ? '✅ 已设置' : '❌ 未设置'}`);
            console.log(`      发现密钥: ${device.disco_key ? '✅ 已设置' : '❌ 未设置'}`);
            console.log(`      授权状态: ${device.authorized ? '✅ 已授权' : '❌ 未授权'}`);
            
            // 检查网络信息
            console.log('   🌐 网络信息:');
            
            let ipAddresses = [];
            if (device.ip_addresses) {
                try {
                    ipAddresses = JSON.parse(device.ip_addresses);
                } catch (e) {
                    ipAddresses = [device.ip_addresses];
                }
            }
            console.log(`      IP地址: ${ipAddresses.length > 0 ? '✅ ' + ipAddresses.join(', ') : '❌ 未分配'}`);
            console.log(`      DERP服务器: ${device.derp ? '✅ ' + device.derp : '❌ 未设置'}`);
            
            // 检查时间信息
            console.log('   ⏰ 时间信息:');
            console.log(`      最后在线: ${device.last_seen ? '✅ ' + device.last_seen : '❌ 未记录'}`);
            console.log(`      创建时间: ${device.created_at ? '✅ ' + device.created_at : '❌ 未记录'}`);
            
            // 检查是否有缺失的关键信息
            const missingFields = [];
            if (!device.node_key) missingFields.push('节点密钥');
            if (!device.machine_key) missingFields.push('机器密钥');
            if (!ipAddresses.length) missingFields.push('IP地址');
            if (!device.derp) missingFields.push('DERP服务器');
            
            if (missingFields.length > 0) {
                console.log(`   ⚠️  缺失字段: ${missingFields.join(', ')}`);
                hasCompleteData = false;
            } else {
                console.log('   ✅ 所有关键字段完整');
            }
            
            console.log('');
        }
        
        return hasCompleteData;
        
    } catch (error) {
        console.log(`❌ 设备详情测试异常: ${error.message}`);
        return false;
    }
}

// 测试前端数据转换
async function testDataTransformation() {
    console.log('🔄 测试前端数据转换...\n');
    
    try {
        // 模拟后端返回的原始数据
        const rawDevice = {
            id: 1,
            hostname: "test-device",
            given_name: "测试设备",
            node_key: "nodekey:abc123...",
            machine_key: "mkey:def456...",
            disco_key: "discokey:ghi789...",
            ip_addresses: '["100.64.0.1", "fd7a:115c:a1e0::1"]',
            derp: "1",
            authorized: true,
            online: true,
            last_seen: "2025-01-30T16:30:00Z",
            created_at: "2025-01-30T16:00:00Z",
            tags: '["web", "production"]',
            advertise_routes: '["192.168.1.0/24"]',
            endpoints: '["192.168.1.100:41641"]'
        };
        
        console.log('📝 原始后端数据:');
        console.log(`   IP地址字段: ${rawDevice.ip_addresses}`);
        console.log(`   标签字段: ${rawDevice.tags}`);
        console.log(`   路由字段: ${rawDevice.advertise_routes}\n`);
        
        // 模拟前端转换逻辑
        let ipAddresses = [];
        if (rawDevice.ip_addresses) {
            try {
                ipAddresses = JSON.parse(rawDevice.ip_addresses);
            } catch (e) {
                ipAddresses = [rawDevice.ip_addresses];
            }
        }
        
        let tags = [];
        if (rawDevice.tags) {
            try {
                tags = JSON.parse(rawDevice.tags);
            } catch (e) {
                tags = [];
            }
        }
        
        let routes = [];
        if (rawDevice.advertise_routes) {
            try {
                routes = JSON.parse(rawDevice.advertise_routes);
            } catch (e) {
                routes = [];
            }
        }
        
        console.log('🔄 转换后的前端数据:');
        console.log(`   IP地址数组: [${ipAddresses.join(', ')}]`);
        console.log(`   标签数组: [${tags.join(', ')}]`);
        console.log(`   路由数组: [${routes.join(', ')}]`);
        console.log(`   节点密钥: ${rawDevice.node_key}`);
        console.log(`   机器密钥: ${rawDevice.machine_key}`);
        console.log(`   DERP服务器: ${rawDevice.derp}\n`);
        
        // 验证转换结果
        const validations = [
            { name: 'IP地址解析', passed: ipAddresses.length === 2 },
            { name: '标签解析', passed: tags.length === 2 },
            { name: '路由解析', passed: routes.length === 1 },
            { name: '密钥字段', passed: rawDevice.node_key && rawDevice.machine_key },
            { name: 'DERP字段', passed: rawDevice.derp === "1" }
        ];
        
        let allPassed = true;
        for (const validation of validations) {
            const status = validation.passed ? '✅ 通过' : '❌ 失败';
            console.log(`   ${validation.name}: ${status}`);
            if (!validation.passed) allPassed = false;
        }
        
        return allPassed;
        
    } catch (error) {
        console.log(`❌ 数据转换测试异常: ${error.message}`);
        return false;
    }
}

// 主测试函数
async function main() {
    console.log('🚀 设备详情显示测试开始...\n');
    console.log('=' .repeat(50));
    
    const results = [];
    
    // 测试设备详情数据
    const detailsResult = await testDeviceDetails();
    results.push({ name: '设备详情数据', passed: detailsResult });
    
    // 测试数据转换
    const transformResult = await testDataTransformation();
    results.push({ name: '数据转换逻辑', passed: transformResult });
    
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
        console.log('\n🎉 所有测试通过！设备详情显示功能正常！');
    } else {
        console.log('\n⚠️  部分测试失败，设备详情可能显示不完整。');
    }
}

// 运行测试
main().catch(console.error);
