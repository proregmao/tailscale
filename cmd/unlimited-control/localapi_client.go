package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"runtime"
	"time"
)

// LocalAPIClient LocalAPI客户端
type LocalAPIClient struct {
	client     *http.Client
	socketPath string
}

// NewLocalAPIClient 创建LocalAPI客户端
func NewLocalAPIClient() *LocalAPIClient {
	socketPath := getLocalAPISocketPath()
	
	var transport http.RoundTripper
	
	if runtime.GOOS == "windows" {
		// Windows使用TCP连接
		transport = &http.Transport{
			Dial: func(network, addr string) (net.Conn, error) {
				return net.Dial("tcp", "127.0.0.1:41112")
			},
		}
	} else {
		// Unix系统使用Unix socket
		transport = &http.Transport{
			Dial: func(network, addr string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		}
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	return &LocalAPIClient{
		client:     client,
		socketPath: socketPath,
	}
}

// Get 发送GET请求
func (c *LocalAPIClient) Get(path string) (map[string]interface{}, error) {
	url := fmt.Sprintf("http://local-tailscaled.sock%s", path)
	
	resp, err := c.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}

// Post 发送POST请求
func (c *LocalAPIClient) Post(path string, data interface{}) (map[string]interface{}, error) {
	url := fmt.Sprintf("http://local-tailscaled.sock%s", path)
	
	var body io.Reader
	if data != nil {
		jsonData, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal data: %w", err)
		}
		body = bytes.NewReader(jsonData)
	}

	resp, err := c.client.Post(url, "application/json", body)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}

// Status 获取状态
func (c *LocalAPIClient) Status() (map[string]interface{}, error) {
	return c.Get("/localapi/v0/status")
}

// Health 获取健康状态
func (c *LocalAPIClient) Health() (map[string]interface{}, error) {
	return c.Get("/localapi/v0/health")
}

// Prefs 获取偏好设置
func (c *LocalAPIClient) Prefs() (map[string]interface{}, error) {
	return c.Get("/localapi/v0/prefs")
}

// Up 启动网络
func (c *LocalAPIClient) Up(options map[string]interface{}) (map[string]interface{}, error) {
	return c.Post("/localapi/v0/up", options)
}

// Down 停止网络
func (c *LocalAPIClient) Down() (map[string]interface{}, error) {
	return c.Post("/localapi/v0/down", nil)
}

// Ping 执行ping测试
func (c *LocalAPIClient) Ping(ip string) (map[string]interface{}, error) {
	data := map[string]string{"ip": ip}
	return c.Post("/localapi/v0/ping", data)
}

// WhoIs 查询IP对应的设备信息
func (c *LocalAPIClient) WhoIs(ip string) (map[string]interface{}, error) {
	return c.Get(fmt.Sprintf("/localapi/v0/whois?addr=%s", ip))
}

// FileTargets 获取文件传输目标
func (c *LocalAPIClient) FileTargets() ([]interface{}, error) {
	result, err := c.Get("/localapi/v0/file-targets")
	if err != nil {
		return nil, err
	}

	if targets, ok := result["targets"].([]interface{}); ok {
		return targets, nil
	}

	return []interface{}{}, nil
}

// TestLocalAPIClient 测试LocalAPI客户端功能
func TestLocalAPIClient() {
	fmt.Println("🧪 Testing LocalAPI Client...")

	client := NewLocalAPIClient()

	// 测试健康检查
	fmt.Println("\n📊 Testing Health Check...")
	health, err := client.Health()
	if err != nil {
		fmt.Printf("❌ Health check failed: %v\n", err)
	} else {
		fmt.Printf("✅ Health check passed: %v\n", health["overall"])
	}

	// 测试状态查询
	fmt.Println("\n📊 Testing Status Query...")
	status, err := client.Status()
	if err != nil {
		fmt.Printf("❌ Status query failed: %v\n", err)
	} else {
		fmt.Printf("✅ Status query successful\n")
		fmt.Printf("   Backend State: %v\n", status["BackendState"])
		fmt.Printf("   Version: %v\n", status["Version"])
		if peers, ok := status["Peers"].(map[string]interface{}); ok {
			fmt.Printf("   Peers Count: %d\n", len(peers))
		}
	}

	// 测试偏好设置
	fmt.Println("\n📊 Testing Preferences...")
	prefs, err := client.Prefs()
	if err != nil {
		fmt.Printf("❌ Preferences query failed: %v\n", err)
	} else {
		fmt.Printf("✅ Preferences query successful\n")
		fmt.Printf("   Control URL: %v\n", prefs["ControlURL"])
		fmt.Printf("   Want Running: %v\n", prefs["WantRunning"])
	}

	// 测试文件目标
	fmt.Println("\n📊 Testing File Targets...")
	targets, err := client.FileTargets()
	if err != nil {
		fmt.Printf("❌ File targets query failed: %v\n", err)
	} else {
		fmt.Printf("✅ File targets query successful\n")
		fmt.Printf("   Available targets: %d\n", len(targets))
	}

	fmt.Println("\n🎉 LocalAPI Client test completed!")
}

// 命令行工具入口
func runLocalAPITool() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: ./unlimited-control localapi <command>")
		fmt.Println("Commands:")
		fmt.Println("  test     - Run LocalAPI tests")
		fmt.Println("  status   - Get status")
		fmt.Println("  health   - Get health")
		fmt.Println("  prefs    - Get preferences")
		fmt.Println("  up       - Start network")
		fmt.Println("  down     - Stop network")
		fmt.Println("  ping <ip> - Ping an IP")
		fmt.Println("  whois <ip> - Get device info for IP")
		return
	}

	command := os.Args[2]
	client := NewLocalAPIClient()

	switch command {
	case "test":
		TestLocalAPIClient()
	case "status":
		result, err := client.Status()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		printJSON(result)
	case "health":
		result, err := client.Health()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		printJSON(result)
	case "prefs":
		result, err := client.Prefs()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		printJSON(result)
	case "up":
		result, err := client.Up(map[string]interface{}{})
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		printJSON(result)
	case "down":
		result, err := client.Down()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		printJSON(result)
	case "ping":
		if len(os.Args) < 4 {
			fmt.Println("Usage: ./unlimited-control localapi ping <ip>")
			return
		}
		ip := os.Args[3]
		result, err := client.Ping(ip)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		printJSON(result)
	case "whois":
		if len(os.Args) < 4 {
			fmt.Println("Usage: ./unlimited-control localapi whois <ip>")
			return
		}
		ip := os.Args[3]
		result, err := client.WhoIs(ip)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		printJSON(result)
	default:
		fmt.Printf("Unknown command: %s\n", command)
	}
}

// printJSON 格式化打印JSON
func printJSON(data interface{}) {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		fmt.Printf("Error formatting JSON: %v\n", err)
		return
	}
	fmt.Println(string(jsonData))
}
