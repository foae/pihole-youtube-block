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
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Alternative regex: ^r[0-9]+-*sn-[A-Za-z0-9]*-*.googlevideo.com$
var rgx = regexp.MustCompile(`(?m)r([0-9])---sn-(.*?)\.googlevideo\.com`)

// Config describes the configurable options for this program.
type Config struct {
	LogsDirectory           string `json:"PIHOLE_LOGS_DIR"`
	LogFileNamePrefix       string `json:"LOG_FILE_NAME_PREFIX"`
	OutputFileName          string `json:"COMPILED_FILE_NAME"`
	PopConfirmationDialogue bool   `json:"POP_CONFIRMATION_DIALOGUE"`
}

// DomainMap holds the gathered domains from the log files.
// The underlying map consists of key: domain, value: number of occurrences.
type DomainMap struct {
	m map[string]int
	l sync.Locker
}

func main() {
	ts := time.Now()
	lock := new(sync.Mutex)

	cfg, err := NewConfig()
	if err != nil {
		log.Fatalf("unable to start: %v", err)
	}

	// Read all files from the configured `LogsDirectory`
	files, err := ioutil.ReadDir(cfg.LogsDirectory)
	if err != nil {
		log.Fatalf("could not read files from the configured directory (%v): %v", cfg.LogsDirectory, err)
	}

	// Filter through the files.
	filesOfInterest := make([]string, 0, 1024)
	for _, f := range files {
		switch {
		case f.IsDir():
			continue
		case strings.HasPrefix(f.Name(), "pihole.log"):
			filesOfInterest = append(filesOfInterest, f.Name())
		}
	}

	// Keep track of all gathered domains.
	compiledMap := NewDomainMap(lock)
	var wg sync.WaitGroup
	wg.Add(len(filesOfInterest))

	// For each file of interest, read it line-by-line.
	for _, fileName := range filesOfInterest {
		f := cfg.LogsDirectory + fileName
		go processFile(f, compiledMap, &wg)
	}

	fmt.Println(">>> Waiting for all jobs to finish...")
	wg.Wait()

	totalCollectedDomains := len(compiledMap.Domains())
	fmt.Printf(">>> Done: (%v) unique extracted domains written to (%v) in (%v)\n",
		totalCollectedDomains,
		cfg.OutputFileName,
		time.Since(ts),
	)

	// Write to a file the gathered domains.
	// TODO: maybe give the option to append if file exists and not overwrite?
	f, err := os.Create("./" + cfg.OutputFileName)
	if err != nil {
		log.Fatalf("could not write output to file (%v)", cfg.OutputFileName)
	}

	for domain, _ := range compiledMap.Domains() {
		if _, err := f.WriteString(domain + "\n"); err != nil {
			log.Printf("skipped: could not write domain (%v) to file (%v): %v", domain, cfg.OutputFileName, err)
			continue
		}
	}

	// Directly send the found domains to pihole, if the config says so.
	if cfg.PopConfirmationDialogue == false {
		log.Printf("Automatically adding (%v) domains to the blacklist...", totalCollectedDomains)

		out, err := execPihole(compiledMap.DomainsToString())
		if err != nil {
			log.Fatalf("could not send `blacklist domains` command to pihole: %v", err)
		}

		log.Printf("Output from pihole: %s", out)
		log.Println("Finished.")
		os.Exit(0)
	}

	// Otherwise pop up a confirmation dialogue.
	r := bufio.NewReader(os.Stdin)
	fmt.Println("-----------")
	fmt.Printf("Would you like to stick those (%v) collected domains into *your* pihole? (y/n)\n",
		totalCollectedDomains,
	)
	fmt.Println("-----------")

	for {
		rn, _, err := r.ReadRune()
		switch {
		case err != nil:
			log.Fatalf("could not read input: %v", err)
		case rn == 'Y', rn == 'y':
			log.Println("> Yes. Please wait.")
			log.Printf("Adding (%v) domains to the blacklist...", totalCollectedDomains)

			out, err := execPihole(compiledMap.DomainsToString())
			if err != nil {
				log.Fatalf("could not send `blacklist domains` command to pihole: %v", err)
			}

			log.Printf("Output from pihole: %s", out)
			log.Println("Finished.")
			os.Exit(0)
		case rn == 'N', rn == 'n':
			log.Println("No is a no. Bye.")
			os.Exit(0)
		default:
			log.Printf("Your key (%v) is not supported. Use: Y, y, N, n", rn)
		}
	}
}

// Insert takes care of adding domains the the domain map.
func (dm DomainMap) Insert(s string) {
	dm.l.Lock()
	dm.m[s]++
	dm.l.Unlock()
}

// Len ...
func (dm DomainMap) Len() int {
	return len(dm.m)
}

// Domains returns the underlying domain map.
func (dm DomainMap) Domains() map[string]int {
	dm.l.Lock()
	defer dm.l.Unlock()

	return dm.m
}

// DomainsToString returns the gathered domains into a single string, space separated.
func (dm DomainMap) DomainsToString() string {
	dm.l.Lock()

	var d strings.Builder
	for domain, _ := range dm.m {
		d.WriteString(domain + " ")
	}

	dm.l.Unlock()
	return d.String()
}

// NewDomainMap returns a pointer to a `DomainMap`.
func NewDomainMap(l sync.Locker) *DomainMap {
	return &DomainMap{
		m: make(map[string]int, 0),
		l: l,
	}
}

// NewConfig reads the JSON config file and returns it as a struct.
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

func processFile(f string, registry *DomainMap, wg *sync.WaitGroup) error {
	defer wg.Done()

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

LineLoop:
	for {
		line, lineTooLong, err := r.ReadLine()
		switch {
		case err == io.EOF:
			break LineLoop
		case err != nil:
			log.Printf("Skipped unreadable file (%v): %v", f, err)
			continue
		case lineTooLong:
			log.Printf("Skipped line (%v) in file (%v). Line is too long.", lineNumber, f)
			continue
		}

		for _, m := range rgx.FindAll(line, -1) {
			s := fmt.Sprintf("%s", m)
			registry.Insert(s)
		}

		lineNumber++
	}

	log.Printf("Finished processing file (%v).", f)

	return nil
}

func execPihole(s string) ([]byte, error) {
	var cmd *exec.Cmd
	cmd = exec.Command("bash", "-c", "pihole -b "+s)
	return cmd.CombinedOutput()
}
