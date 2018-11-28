package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
)

// Config describes the configurable options for this program.
type Config struct {
	LogsDirectory     string `json:"PIHOLE_LOGS_DIR"`
	LogFileNamePrefix string `json:"LOG_FILE_NAME_PREFIX"`
	OutputFileName    string `json:"COMPILED_FILE_NAME"`
}

func main() {
	cfg, err := NewConfig()
	if err != nil {
		log.Fatalf("unable to start: %v", err)
	}

	// Read all files from `LogsDirectory`
	files, err := ioutil.ReadDir(cfg.LogsDirectory)
	if err != nil {
		log.Fatalf("could not read files from the configured directory (%v): %v", cfg.LogsDirectory, err)
	}

	// Filter through the files.
	filesOfInterest := make([]string, 0)
	for _, f := range files {
		switch {
		case f.IsDir():
			continue
		case strings.HasPrefix(f.Name(), "pihole.log"):
			filesOfInterest = append(filesOfInterest, f.Name())
		}
	}

	// Keep track of all gathered domains.
	//compiledMap := make(map[string]int, 0)

	// For each file of interest, read it line-by-line.
	for _, fileName := range filesOfInterest {
		file := cfg.LogsDirectory + fileName
		f, err := os.Open(file)
		if err != nil {
			log.Printf("Skipped unreadable file (%v): %v", f, err)
			continue
		}

		r := bufio.NewReader(f)
		var lineNumber int
		for {
			line, lineTooLong, err := r.ReadLine()
			if err != nil {
				log.Printf("Skipped unreadable file (%v): %v", f, err)
				continue
			}

			if lineTooLong {
				log.Printf("Skipped line (%v) in file (%v). Line is too long.", lineNumber, file)
				continue
			}

			// With `line`, read if there's a pattern
			// rXXX.googlevideo.com

			lineNumber++
		}

	}

	// If file has .gz extension, use a different bufio to read with
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
