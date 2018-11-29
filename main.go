package main

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

var rgx = regexp.MustCompile(`(?m)r([0-9])(.*?)\.googlevideo\.com`)
var lock = sync.Mutex{}

// Config describes the configurable options for this program.
type Config struct {
	LogsDirectory     string `json:"PIHOLE_LOGS_DIR"`
	LogFileNamePrefix string `json:"LOG_FILE_NAME_PREFIX"`
	OutputFileName    string `json:"COMPILED_FILE_NAME"`
}

type DomainMap struct {
	m map[string]int
}

func (dm DomainMap) Insert(s string) {
	lock.Lock()
	dm.m[s]++
	lock.Unlock()
}

func (dm DomainMap) Domains() map[string]int {
	return dm.m
}

func NewDomainMap() *DomainMap {
	return &DomainMap{
		m: make(map[string]int, 0),
	}
}

func main() {
	ts := time.Now()
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
	compiledMap := NewDomainMap()

	// For each file of interest, read it line-by-line.
	for _, fileName := range filesOfInterest {
		f := cfg.LogsDirectory + fileName
		go processFile(f, compiledMap)
	}

	time.Sleep(time.Second * 2)

	fmt.Printf("Unique extracted domains: %v in %v\n", len(compiledMap.Domains()), time.Since(ts))
}

func processFile(f string, registry *DomainMap) error {
	ts := time.Now()
	openFile, err := os.Open(f)
	if err != nil {
		return fmt.Errorf("processFile: skipped unreadable file (%v): %v", f, err)
	}
	defer openFile.Close()

	var r *bufio.Reader
	if strings.HasSuffix(f, ".gz") {
		rr, err := gzip.NewReader(openFile)
		if err != nil {
			return err
		}
		defer rr.Close()
		r = bufio.NewReader(rr)
	} else {
		r = bufio.NewReader(openFile)
	}

	var lineNumber int

ForEachLine:
	for {
		line, lineTooLong, err := r.ReadLine()
		switch {
		case err == io.EOF:
			log.Printf("Finished reading file (%v) after (%v) lines", f, lineNumber)
			break ForEachLine
		case err != nil:
			log.Printf("Skipped unreadable file (%v): %v", f, err)
			continue
		case lineTooLong:
			log.Printf("Skipped line (%v) in file (%v). Line is too long.", lineNumber, f)
			continue
		}

		// Non-regex version, 2x faster
		/*
		if bytes.Contains(line, []byte(".googlevideo.com")) {
			sLine := bytes.Split(line, []byte(" "))
			// TODO: progressive checking for prefix `.googlevideo.com` from last index in `sLine`
			s := fmt.Sprintf("%s", sLine[len(sLine)-3:len(sLine)-2])
			registry.Insert(s)
		}
		*/

		for _, m := range rgx.FindAll(line, -1) {
			s := fmt.Sprintf("%s", m)
			registry.Insert(s)
		}

		lineNumber++
	}

	log.Printf("Finished processing file (%v) in (%v).", f, time.Since(ts))

	return nil
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

	return &cfg, nil
}
