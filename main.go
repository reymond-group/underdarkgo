package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

type RequestMessage struct {
	Command string   `json:"cmd"`
	Content []string `json:"msg"`
}

type InitResponseMessage struct {
	Command string        `json:"cmd"`
	Content Configuration `json:"msg"`
}

type VariantResponseMessage struct {
	Command string `json:"cmd"`
	Content string `json:"msg"`
	Id      string `json:"id"`
}

type MapResponseMessage struct {
	Command string `json:"cmd"`
	Content string `json:"msg"`
	Id      string `json:"id"`
}

type BinPreviewResponseMessage struct {
	Command string `json:"cmd"`
	Smiles  string `json:"smiles"`
	Index   string `json:"index"`
	BinSize string `json:"binSize"`
}

type BinResponseMessage struct {
	Command string   `json:"cmd"`
	Smiles  []string `json:"smiles"`
	Ids     []string `json:"ids"`
	Coords  []string `json:"coordinates"`
	Fps     []string `json:"fps"`
	Index   string   `json:"index"`
	BinSize string   `json:"binSize"`
}

type SearchResponseMessage struct {
	Command     string     `json:"cmd"`
	BinIndices  [][]uint32 `json:"binIndices"`
	SearchTerms []string   `json:"searchTerms"`
}

type ColorMap struct {
	Id          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	MapFile     string   `json:"mapFile"`
	DataTypes   []string `json:"dataTypes"`
}

type Variant struct {
	Id              string     `json:"id"`
	Name            string     `json:"name"`
	Description     string     `json:"description"`
	Resolution      int        `json:"resolution"`
	DataTypes       []string   `json:"dataTypes"`
	Directory       string     `json:"directory"`
	IndicesFile     string     `json:"indicesFile"`
	CoordinatesFile string     `json:"coordinatesFile"`
	ColorMaps       []ColorMap `json:"maps"`
}

type Fingerprint struct {
	Id              string    `json:"id"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	Directory       string    `json:"directory"`
	InfosFile       string    `json:"infosFile"`
	InfoIndicesFile string    `json:"infoIndicesFile"`
	Variants        []Variant `json:"variants"`
}

type Database struct {
	Id           string        `json:"id"`
	Name         string        `json:"name"`
	Description  string        `json:"description"`
	Directory    string        `json:"directory"`
	Fingerprints []Fingerprint `json:"fingerprints"`
}

type Configuration struct {
	Databases []Database `json:"databases"`
}

var dataDir string
var config Configuration

var variantIndices = map[string][][]uint32{}
var infoOffsets = map[string][]uint32{}
var infoLengths = map[string][]uint32{}

// Allow fast access by id
var databases = map[string]Database{}
var fingerprints = map[string]Fingerprint{}
var variants = map[string]Variant{}
var colorMaps = map[string]ColorMap{}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func underdarkInit(data []string) InitResponseMessage {
	return InitResponseMessage{
		Command: "init",
		Content: config,
	}
}

func underdarkLoadVariant(data []string) VariantResponseMessage {
	variantId := data[0]
	buf, err := ioutil.ReadFile(variants[variantId].CoordinatesFile)

	if err != nil {
		log.Fatal(err)
	}

	return VariantResponseMessage{
		Command: "load:variant",
		Content: string(buf),
		Id:      variantId,
	}
}

func underdarkLoadMap(data []string) MapResponseMessage {
	colorMapId := data[0]

	buf, err := ioutil.ReadFile(colorMaps[colorMapId].MapFile)

	if err != nil {
		log.Fatal(err)
	}

	return MapResponseMessage{
		Command: "load:map",
		Content: string(buf),
		Id:      colorMapId,
	}
}

func underdarkLoadBinPreview(data []string) BinPreviewResponseMessage {
	// databaseId := data[0]
	fingerprintId := data[1]
	variantId := data[2]
	binIndex, _ := strconv.Atoi(data[3])

	file, err := os.Open(fingerprints[fingerprintId].InfosFile)

	if err != nil {
		log.Fatal(err)
		return BinPreviewResponseMessage{}
	}

	defer file.Close()

	// Get the indices in the bin
	compounds := variantIndices[variantId][binIndex]

	if len(compounds) < 1 {
		log.Println("No compounds found at binIndex " + strconv.Itoa(binIndex))
		return BinPreviewResponseMessage{
			Command: "load:binpreview",
			Smiles:  "",
			Index:   "",
			BinSize: "0",
		}
	}

	infoOffset := infoOffsets[fingerprintId][compounds[0]]
	infoLength := infoLengths[fingerprintId][compounds[0]]

	buf := make([]byte, int64(infoLength))
	rn, err := file.ReadAt(buf, int64(infoOffset))

	return BinPreviewResponseMessage{
		Command: "load:binpreview",
		Smiles:  strings.Split(string(buf[:rn-1]), " ")[1],
		Index:   data[3],
		BinSize: strconv.Itoa(len(compounds)),
	}
}

func underdarkLoadBin(data []string) BinResponseMessage {
	// databaseId := data[0]
	fingerprintId := data[1]
	variantId := data[2]
	binIndex, _ := strconv.Atoi(data[3])

	infoFile, err := os.Open(fingerprints[fingerprintId].InfosFile)

	if err != nil {
		log.Fatal(err)
	}

	defer infoFile.Close()

	// Get the indices in the bin
	compounds := variantIndices[variantId][binIndex]

	length := len(compounds)
	ids := make([]string, length)
	smiles := make([]string, length)
	fps := make([]string, length)
	coords := make([]string, length)

	for i := 0; i < length; i++ {
		infoOffset := infoOffsets[fingerprintId][compounds[i]]
		infoLength := infoLengths[fingerprintId][compounds[i]]

		buf := make([]byte, int64(infoLength))
		rn, err := infoFile.ReadAt(buf, int64(infoOffset))
		info := string(buf[:rn-1])
		infos := strings.Split(info, " ")

		ids[i] = infos[0]
		smiles[i] = infos[1]
		fps[i] = infos[2]
		coords[i] = infos[3]

		if err != nil {
			log.Println(err)
		}
	}

	return BinResponseMessage{
		Command: "load:bin",
		Smiles:  smiles,
		Ids:     ids,
		Coords:  coords,
		Fps:     fps,
		Index:   data[3],
		BinSize: strconv.Itoa(len(compounds)),
	}
}

func underdarkSearch(data []string) SearchResponseMessage {
	// The first two strings are the fingerprint and variant ids,
	// from there on, the strings are search queries
	fingerprintId := data[0]
	variantId := data[1]
	searchTerms := data[2:len(data)]

	filteredSearchTerms := filterSearchTerms(searchTerms)

	result, err := search(fingerprintId, variantId, filteredSearchTerms)

	if err != nil {
		log.Print("Error while searching:", err)
		return SearchResponseMessage{
			Command:     "search:infos",
			BinIndices:  nil,
			SearchTerms: filteredSearchTerms,
		}
	}

	return SearchResponseMessage{
		Command:     "search:infos",
		BinIndices:  result,
		SearchTerms: filteredSearchTerms,
	}
}

func serveUnderdark(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)

	if err != nil {
		log.Print("Error during upgrade:", err)
		return
	}

	defer c.Close()

	for {
		msg := RequestMessage{}
		err := c.ReadJSON(&msg)

		if err != nil {
			log.Println("Error while reading message:", err)
			break
		}

		switch msg.Command {
		case "init":
			err = c.WriteJSON(underdarkInit(msg.Content))
		case "load:variant":
			err = c.WriteJSON(underdarkLoadVariant(msg.Content))
		case "load:map":
			err = c.WriteJSON(underdarkLoadMap(msg.Content))
		case "load:binpreview":
			err = c.WriteJSON(underdarkLoadBinPreview(msg.Content))
		case "load:bin":
			err = c.WriteJSON(underdarkLoadBin(msg.Content))
		case "search:infos":
			err = c.WriteJSON(underdarkSearch(msg.Content))
		}

		if err != nil {
			log.Println("Error while responding:", err)
			break
		}
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Example: " + os.Args[0] + " <data-path>")
		os.Exit(1)
	}

	dataDir = os.Args[1]
	config = loadConfig()

	checkConfig()
	loadIndices()

	http.Handle("/", http.FileServer(http.Dir("./assets")))
	http.HandleFunc("/underdark", serveUnderdark)

	log.Println("Serving at localhost:8080 ...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func loadIndices() {
	loopConfig(func(database *Database, path string) {
		// Nothing to do here

	}, func(fingerprint *Fingerprint, path string) {
		// Loading info indices and lengths
		infosLength, _ := countLines(fingerprint.InfoIndicesFile)

		infoOffsets[fingerprint.Id] = make([]uint32, infosLength)
		infoLengths[fingerprint.Id] = make([]uint32, infosLength)

		log.Println("Reading " + fingerprint.InfoIndicesFile + " ...")

		err := readIndexFile(fingerprint.InfoIndicesFile, infoOffsets[fingerprint.Id], infoLengths[fingerprint.Id])

		if err != nil {
			log.Fatal(err)
		}

	}, func(variant *Variant, path string) {
		// Loading the bin contents (indices pointing to the
		// smiles and ids

		indicesLength, _ := countLines(variant.IndicesFile)

		variantIndices[variant.Id] = make([][]uint32, indicesLength)

		log.Println("Reading " + variant.IndicesFile + " ...")

		err := readVariantIndexFile(variant.IndicesFile, variant.Id)
		if err != nil {
			log.Fatal(err)
		}

	}, func(colorMap *ColorMap, path string) {
		// Nothing to do here
	}, false, false)
}

func loadConfig() Configuration {
	if !strings.HasSuffix(dataDir, "/") {
		dataDir += "/"
	}

	buffer, err := ioutil.ReadFile(dataDir + "config.json")

	if err != nil {
		log.Fatal(err)
	}

	var config Configuration
	err = json.Unmarshal(buffer, &config)

	if err != nil {
		log.Fatal(err)
	}

	return config
}

// Also prepends the paths to the file names
func checkConfig() {
	dataDirExists, _ := exists(dataDir)
	if !dataDirExists {
		fmt.Println("The data directory '" + dataDir + "' does not exist.")
		os.Exit(1)
	}

	var nf []string

	loopConfig(func(database *Database, path string) {
		databases[database.Id] = *database
	}, func(fingerprint *Fingerprint, path string) {
		fingerprint.InfosFile = path + fingerprint.InfosFile

		if exists, _ := exists(fingerprint.InfosFile); !exists {
			nf = append(nf, fingerprint.InfosFile)
		}

		fingerprints[fingerprint.Id] = *fingerprint

		fingerprint.InfoIndicesFile = path + fingerprint.InfoIndicesFile

		if exists, _ := exists(fingerprint.InfoIndicesFile); !exists {
			nf = append(nf, fingerprint.InfoIndicesFile)
		}

		fingerprints[fingerprint.Id] = *fingerprint

	}, func(variant *Variant, path string) {
		variant.IndicesFile = path + variant.IndicesFile
		variant.CoordinatesFile = path + variant.CoordinatesFile

		if exists, _ := exists(variant.IndicesFile); !exists {
			nf = append(nf, variant.IndicesFile)
		}

		if exists, _ := exists(variant.CoordinatesFile); !exists {
			nf = append(nf, variant.CoordinatesFile)
		}

		variants[variant.Id] = *variant

	}, func(colorMap *ColorMap, path string) {
		colorMap.MapFile = path + colorMap.MapFile

		if exists, _ := exists(colorMap.MapFile); !exists {
			nf = append(nf, colorMap.MapFile)
		}

		colorMaps[colorMap.Id] = *colorMap
	}, true, true)

	if len(nf) > 0 {
		fmt.Println("The following files were not found. Please add the files or remove the entries from the config.")
	}

	for _, element := range nf {
		fmt.Println(element)
	}

	if len(nf) > 0 {
		os.Exit(1)
	}
}

func loopConfig(databaseCallback func(*Database, string),
	fingerprintCallback func(*Fingerprint, string),
	variantCallback func(*Variant, string),
	colorMapCallback func(*ColorMap, string),
	updatePath bool, updateId bool) {
	for i, _ := range config.Databases {
		database := &config.Databases[i]
		var databasePath string
		if updatePath {
			databasePath = concatPath(dataDir, database.Directory)
			databaseCallback(database, databasePath)
		} else {
			databaseCallback(database, database.Directory)
		}
		for j, _ := range database.Fingerprints {
			fingerprint := &database.Fingerprints[j]
			var fingerprintPath string
			if updateId {
				fingerprint.Id = database.Id + "." + fingerprint.Id
			}
			if updatePath {
				fingerprintPath = concatPath(databasePath, fingerprint.Directory)
				fingerprintCallback(fingerprint, fingerprintPath)
			} else {
				fingerprintCallback(fingerprint, fingerprint.Directory)
			}

			for k, _ := range fingerprint.Variants {
				variant := &fingerprint.Variants[k]
				var variantPath string
				if updateId {
					variant.Id = fingerprint.Id + "." + variant.Id
				}
				if updatePath {
					variantPath = concatPath(fingerprintPath, variant.Directory)
					variantCallback(variant, variantPath)
				} else {
					variantCallback(variant, variant.Directory)
				}

				for l, _ := range variant.ColorMaps {
					colorMap := &variant.ColorMaps[l]
					if updateId {
						colorMap.Id = variant.Id + "." + colorMap.Id
					}
					if updatePath {
						colorMapCallback(colorMap, variantPath)
					} else {
						colorMapCallback(colorMap, variant.Directory)
					}

				}
			}
		}
	}
}

func concatPath(a string, b string) string {
	if !strings.HasSuffix(a, "/") {
		a += "/"
	}
	if strings.HasPrefix(b, "/") {
		strings.TrimLeft(b, "/")
	}
	if !strings.HasSuffix(b, "/") {
		b += "/"
	}

	return a + b
}

func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}

func countLines(path string) (int, error) {
	r, err := os.Open(path)

	if err != nil {
		log.Fatal("Could not open file: " + path)
	}

	buf := make([]byte, 32*1024)
	count := 0
	lineSep := []byte{'\n'}

	for {
		c, err := r.Read(buf)
		count += bytes.Count(buf[:c], lineSep)

		if err == io.EOF {
			return count, nil
		} else if err != nil {
			return count, err
		}
	}
}

func readIndexFile(path string, offsets []uint32, lengths []uint32) error {
	r, err := os.Open(path)
	scanner := bufio.NewScanner(r)

	i := 0
	for scanner.Scan() {
		line := scanner.Text()
		values := strings.Split(line, ",")

		offset, _ := strconv.ParseUint(values[0], 10, 32)
		length, _ := strconv.ParseUint(values[1], 10, 16)

		offsets[i] = uint32(offset)
		lengths[i] = uint32(length)

		i++
	}

	return err
}

func readVariantIndexFile(path string, id string) error {
	r, err := os.Open(path)
	defer r.Close()
	scanner := bufio.NewScanner(r)
	scanner.Split(bufio.ScanLines)

	// The buffer in the scanner is to small for large bins, so increase it a bit
	// Set the buffer size to 1024 * 1024 bytes (1 MB) ~ 1 million characters
	const maxCapacity = 1024 * 1024
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	i := 0
	for scanner.Scan() {
		line := scanner.Text()
		values := strings.Split(line, ",")
		n := len(values)
		variantIndices[id][i] = make([]uint32, n)

		for j := 0; j < n; j++ {
			value, _ := strconv.ParseUint(values[j], 10, 32)
			variantIndices[id][i][j] = uint32(value)
		}

		i++
	}

	return err
}

func search(fingerprintId string, variantId string, terms []string) ([][]uint32, error) {
	file, err := os.Open(fingerprints[fingerprintId].InfosFile)

	nLines := len(infoOffsets[fingerprintId])
	nTerms := len(terms)

	results := make([][]uint32, nTerms)
	binIndices := make([][]uint32, nTerms)

	for i := 0; i < nTerms; i++ {
		results[i] = make([]uint32, 0)
		binIndices[i] = make([]uint32, 0)
	}

	for i := 0; i < nLines; i++ {
		buf := make([]byte, int64(infoLengths[fingerprintId][i]))
		rn, _ := file.ReadAt(buf, int64(infoOffsets[fingerprintId][i]))

		val := string(buf[:rn-1])

		for j := 0; j < nTerms; j++ {
			if strings.Contains(val, terms[j]) {
				results[j] = append(results[j], uint32(i))
			}
		}
	}

	// Finding the bins for the line numbers
	nBins := len(variantIndices[variantId])

	for i := 0; i < nBins; i++ {
		for j := 0; j < len(variantIndices[variantId][i]); j++ {
			for k := 0; k < nTerms; k++ {
				for l := 0; l < len(results[k]); l++ {
					if results[k][l] == variantIndices[variantId][i][j] {
						binIndices[k] = append(binIndices[k], uint32(i))
					}
				}
			}
		}
	}

	return binIndices, err
}

func readLine(r *os.File, line int) (string, error) {
	scanner := bufio.NewScanner(r)
	scanner.Split(bufio.ScanLines)

	const maxCapacity = 1024 * 1024
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	var result string

	i := 0
	for scanner.Scan() {
		if i == line {
			result = scanner.Text()
			break
		}
	}

	return result, nil
}

func filterSearchTerms(terms []string) []string {
	filtered := make([]string, 0)

	for _, v := range terms {
		if v != "" {
			filtered = append(filtered, v)
		}
	}

	return filtered
}
