package config

import (
	"os"
	"path/filepath"
	"encoding/json"
)

type Config struct {
	DBURL string `json:"db_url"`
	CurrentUserName string `json:"current_user_name"`
}

func Read() (Config, error) {
	configFilePath, err := getConfigPath()
	if err != nil {
		return Config{}, err
	}

	// read JSON file
	data, err := os.ReadFile(configFilePath)
	if err != nil {
		return Config{}, err
	}

	// fill out and return struct
	var c Config
	err = json.Unmarshal(data, &c)
	if err != nil{
		return Config{}, err
	}
	return c, nil
}

func (cfg *Config) SetUser(username string) error {
	// set username
	cfg.CurrentUserName = username

	// write struct to JSON file
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	configFilePath, err := getConfigPath()
	if err != nil {
		return err
	}
	os.WriteFile(configFilePath, data, 0644)
	if err != nil {
		return err
	}

	return nil
}

func getConfigPath() (string, error) {
	const configFileName = ".gatorconfig.json"
	homeDirPath, err := os.UserHomeDir() 
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDirPath, configFileName), nil
}