package config

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
)

type Config struct {
	Coins       []Coins  `json:"coins"`
	PrivatePath []string `json:"private_path"`
	DebugLevel  int      `json:"debug_level"`
}

type Coins struct {
	Url         string `json:"url"`
	CzzAddress  string `json:"czz_address"`
	EthfAddress string `json:"ethf_address"`
}

func LoadConfig(cfg *Config, filep string) {

	// Default config.
	configFileName := "config.json"
	if len(os.Args) > 1 {
		configFileName = os.Args[1]
	}
	configFileName, _ = filepath.Abs(configFileName)
	log.Printf("Loading config: %v", configFileName)

	if filep != "" {
		configFileName = filep
	}
	configFile, err := os.Open(configFileName)
	if err != nil {
		log.Fatal("File error: ", err.Error())
	}
	defer configFile.Close()
	jsonParser := json.NewDecoder(configFile)
	if err := jsonParser.Decode(&cfg); err != nil {
		log.Fatal("Config error: ", err.Error())
	}
	select {}
}

func (cfg *Config) GetConfig() *Config {
	return cfg
}
