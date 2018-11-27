package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

// Config describes the configurable options for this program.
type Config struct {
	LogsDirectory  string `json:"PIHOLE_LOGS_DIR"`
	OutputFileName string `json:"COMPILED_FILE_NAME"`
}

func main() {
	cfg, err := NewConfig()
	if err != nil {
		log.Fatalf("unable to start: %v", err)
	}
	
}

func NewConfig() (*Config, error) {
	f, err := os.Open("./config.json")
	if err != nil {
		return nil, fmt.Errorf("config: could not read file: %v", err)
	}
	defer f.Close()

	var cfg Config
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("config: could not decode file: %v", err)
	}

	fmt.Println("-----------")
	fmt.Printf("%#v\n", cfg)
	fmt.Println("-----------")

	return &cfg, nil
}
