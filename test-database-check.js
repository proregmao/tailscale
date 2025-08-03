#!/usr/bin/env node

/**
 * 数据库数据检查
 * 直接查看数据库中的设备数据
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

// 检查数据库中的设备数据
async function checkDatabaseData() {
    console.log('🔍 检查数据库中的设备数据...\n');
    
    try {
        // 获取设备列表
        const response = await makeRequest(`${API_BASE}/api/v1/devices`, {
            method: 'GET',
            headers: {
                'Authorization': `Bearer ${API_KEY}`
            }
        });
        
        if (response.status !== 200 || !response.data.data) {
            console.log('❌ 获取设备列表失败');
            return false;
        }
        
        const devices = response.data.data;
        console.log(`📊 数据库中共有 ${devices.length} 个设备\n`);
        
        // 分析每个设备的原始数据
        for (let i = 0; i < devices.length; i++) {
            const device = devices[i];
            console.log(`📱 设备 ${i + 1}: ${device.hostname || '未命名'}`);
            console.log(`   ID: ${device.id}`);
            console.log(`   原始数据结构:`);
            
            // 显示所有字段
            const fields = [
                'node_key', 'machine_key', 'disco_key', 'hostname', 'given_name',
                'user_id', 'ip_addresses', 'endpoints', 'derp', 'last_seen',
                'online', 'authorized', 'tags', 'forced_tags', 'advertise_routes',
                'enabled_routes', 'exit_node', 'exit_node_route', 'created_at', 'updated_at'
            ];
            
            for (const field of fields) {
                const value = device[field];
                const status = value !== null && value !== undefined && value !== '' ? '✅' : '❌';
                let displayValue = value;
                
                // 处理长字符串显示
                if (typeof value === 'string' && value.length > 50) {
                    displayValue = value.substring(0, 30) + '...';
                }
                
                console.log(`      ${field}: ${status} ${displayValue}`);
            }
            
            console.log('');
        }
        
        // 检查最新设备的完整性
        if (devices.length > 0) {
            const latestDevice = devices[devices.length - 1];
            console.log('🔍 最新设备完整性检查:');
            
            const requiredFields = {
                'node_key': '节点密钥',
                'machine_key': '机器密钥', 
                'disco_key': '发现密钥',
                'derp': 'DERP服务器',
                'ip_addresses': 'IP地址',
                'authorized': '授权状态'
            };
            
            let missingCount = 0;
            for (const [field, name] of Object.entries(requiredFields)) {
                const value = latestDevice[field];
                const hasValue = value !== null && value !== undefined && value !== '';
                
                if (hasValue) {
                    console.log(`   ✅ ${name}: 已设置`);
                } else {
                    console.log(`   ❌ ${name}: 未设置`);
                    missingCount++;
                }
            }
            
            console.log(`\n📈 完整性评分: ${((Object.keys(requiredFields).length - missingCount) / Object.keys(requiredFields).length * 100).toFixed(1)}%`);
            
            return missingCount === 0;
        }
        
        return false;
        
    } catch (error) {
        console.log(`❌ 数据库检查异常: ${error.message}`);
        return false;
    }
}

// 创建新设备并检查
async function createAndCheckDevice() {
    console.log('\n🆕 创建新设备并检查数据保存...\n');
    
    try {
        // 生成测试密钥
        const randomBytes = new Uint8Array(32);
        for(let i = 0; i < 32; i++) {
            randomBytes[i] = Math.floor(Math.random() * 256);
        }
        const rawKey = Array.from(randomBytes).map(b => b.toString(16).padStart(2, '0')).join('');
        const nodeKey = `nodekey:${rawKey}`;
        
        console.log(`📝 生成测试密钥: ${nodeKey.substring(0, 20)}...`);
        
        // 创建设备
        const registerRequest = {
            NodeKey: nodeKey,
            Hostinfo: {
                Hostname: `test-check-${Date.now()}`,
                OS: "linux"
            },
            User: "1"
        };
        
        console.log('📤 创建新设备...');
        const response = await makeRequest(`${API_BASE}/machine/register`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${API_KEY}`
            },
            body: JSON.stringify(registerRequest)
        });
        
        if (response.status === 200) {
            console.log('✅ 设备创建成功');
            
            // 等待一下让数据库保存
            await new Promise(resolve => setTimeout(resolve, 1000));
            
            // 重新检查数据库
            return await checkDatabaseData();
        } else {
            console.log(`❌ 设备创建失败: HTTP ${response.status}`);
            return false;
        }
        
    } catch (error) {
        console.log(`❌ 创建设备异常: ${error.message}`);
        return false;
    }
}

// 主函数
async function main() {
    console.log('🚀 数据库数据检查开始...\n');
    console.log('=' .repeat(60));
    
    // 首先检查现有数据
    const currentDataOk = await checkDatabaseData();
    
    console.log('=' .repeat(60));
    
    // 创建新设备并检查
    const newDataOk = await createAndCheckDevice();
    
    console.log('=' .repeat(60));
    console.log('📊 检查结果汇总:');
    console.log(`   现有数据完整性: ${currentDataOk ? '✅ 完整' : '❌ 不完整'}`);
    console.log(`   新设备数据完整性: ${newDataOk ? '✅ 完整' : '❌ 不完整'}`);
    
    if (newDataOk) {
        console.log('\n🎉 数据库保存功能正常！所有字段都正确保存！');
    } else {
        console.log('\n⚠️  数据库保存存在问题，部分字段未正确保存。');
    }
}

// 运行检查
main().catch(console.error);
