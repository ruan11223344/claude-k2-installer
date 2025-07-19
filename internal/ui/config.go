package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type AppConfig struct {
	APIKey string `json:"api_key"`
	RPM    string `json:"rpm"`
}

const configFileName = ".claude-k2-installer-config.json"

// SaveConfig 保存配置到本地文件
func SaveConfig(apiKey, rpm string) error {
	config := AppConfig{
		APIKey: apiKey,
		RPM:    rpm,
	}
	
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	
	configPath, err := getConfigPath()
	if err != nil {
		return err
	}
	
	return os.WriteFile(configPath, data, 0600)
}

// LoadConfig 从本地文件加载配置
func LoadConfig() (*AppConfig, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return nil, err
	}
	
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	
	var config AppConfig
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}
	
	return &config, nil
}

// getConfigPath 获取配置文件路径
func getConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	
	return filepath.Join(home, configFileName), nil
}