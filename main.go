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
	"time"
)

const writeWait = 100 * time.Second
const pongWait = 120 * time.Second
const pingPeriod = (pongWait * 9) / 10

type Client struct {
	conn *websocket.Conn
	send chan RequestMessage
}

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

type StatsResponseMessage struct {
	Command string `json:"cmd"`
	Content Stats  `json:"msg"`
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
	Command 	string   `json:"cmd"`
	Smiles  	[]string `json:"smiles"`
	Ids     	[]string `json:"ids"`
	Coords  	[]string `json:"coordinates"`
	Fps     	[]string `json:"fps"`
	BinIndices	[]uint32 `json:"binIndices"`
	Index   	string   `json:"index"`
	BinSize 	string   `json:"binSize"`
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
	Min             []float32 `json:"min"`
	Max             []float32 `json:"max"`
}

type Database struct {
	Id           string        `json:"id"`
	Name         string        `json:"name"`
	Description  string        `json:"description"`
	Directory    string        `json:"directory"`
	Fingerprints []Fingerprint `json:"fingerprints"`
}

type Stats struct {
	CompoundCount uint32   `json:"compoundCount"`
	BinCount      uint32   `json:"binCount"`
	AvgBinSize    float32  `json:"avgCompoundCount"`
	BinHist       []uint32 `json:"binHist"`
	HistMin       uint32   `json:"histMin"`
	HistMax       uint32   `json:"histMax"`
}

type Configuration struct {
	Databases []Database `json:"databases"`
}

var debug bool

var dataDir string
var config Configuration

var variantIndices = map[string][][]uint32{}
var infoOffsets = map[string][]uint64{}
var infoLengths = map[string][]uint32{}

// Allow fast access by id
var databases = map[string]Database{}
var fingerprints = map[string]Fingerprint{}
var variants = map[string]Variant{}
var colorMaps = map[string]ColorMap{}
var stats = map[string]Stats{}

var upgrader = websocket.Upgrader{
	EnableCompression: true,
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
		fmt.Printf("Error loading variant: %v", err)
	}

	return VariantResponseMessage{
		Command: "load:variant",
		Content: string(buf),
		Id:      variantId,
	}
}

func underdarkLoadStats(data []string) StatsResponseMessage {
	variantId := data[0]

	return StatsResponseMessage{
		Command: "load:stats",
		Content: stats[variantId],
		Id:      variantId,
	}
}

func underdarkLoadMap(data []string) MapResponseMessage {
	colorMapId := data[0]

	buf, err := ioutil.ReadFile(colorMaps[colorMapId].MapFile)

	if err != nil {
		fmt.Printf("Error loading map: %v", err)
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

	if debug {
		fmt.Printf("Bin preview for index %d in file %s\n", binIndex, fingerprints[fingerprintId].InfosFile)
	}

	file, err := os.Open(fingerprints[fingerprintId].InfosFile)

	if err != nil {
		fmt.Printf("Error loading variant, returning empty response: %v\n", err)
		return BinPreviewResponseMessage{}
	}

	defer file.Close()

	// Make sure that the binIndex exists and avoid out of range
	if len(variantIndices[variantId]) <= binIndex {
		fmt.Printf("binIndex %s is out of range.", strconv.Itoa(binIndex))
		return BinPreviewResponseMessage{
			Command: "load:binpreview",
			Smiles:  "",
			Index:   "",
			BinSize: "0",
		}
	}

	// Get the indices in the bin
	compounds := variantIndices[variantId][binIndex]

	if len(compounds) < 1 {
		fmt.Printf("No compounds found at binIndex %s.", strconv.Itoa(binIndex))
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

	line := string(buf[:rn-1])
	smiles := strings.Split(line, " ")

	if len(smiles) < 1 {
		fmt.Printf("No smiles found at binIndex %s. Line content: %s\n", strconv.Itoa(binIndex), line)
		return BinPreviewResponseMessage{
			Command: "load:binpreview",
			Smiles:  "",
			Index:   "",
			BinSize: "0",
		}
	} else {
		if debug {
			fmt.Printf("Loading smiles from offset %d with length %d:\n%s %s\n", int64(infoOffset), int64(infoLength), smiles[0], smiles[1])
			fmt.Printf("Returning smiles %s\n", strings.Split(string(buf[:rn-1]), " ")[1])
		}
	}

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
	binIndices := stringToIntArray(strings.Split(data[3], ","))

	infoFile, err := os.Open(fingerprints[fingerprintId].InfosFile)

	if err != nil {
		fmt.Printf("Error loading bin: %v", err)
	}

	defer infoFile.Close()

	// Check whether binIndex is within range
	if uint32(len(variantIndices[variantId])) <= binIndices[0] {
		fmt.Printf("binIndex %s is out of range.", strconv.FormatUint(uint64(binIndices[0]), 10))
		return BinResponseMessage{
			Command: 	"load:bin",
			Index:   	data[3],
			BinSize: 	"0",
		}
	}

	// Get the indices in the bin
	compounds := variantIndices[variantId][binIndices[0]]
	var compoundBinIndices []uint32

	for i := 0; i < len(compounds); i++ {
		compoundBinIndices = append(compoundBinIndices, binIndices[0])
	}
	
	for i := 1; i < len(binIndices); i++ {
		if uint32(len(variantIndices[variantId])) <= binIndices[i] {
			fmt.Printf("binIndex %s is out of range.", strconv.FormatUint(uint64(binIndices[i]), 10))
			return BinResponseMessage{
				Command: 	"load:bin",
				Index:   	data[3],
				BinSize: 	"0",
			}
		}

		compoundsInBin := variantIndices[variantId][binIndices[i]]
		compounds = append(compounds, compoundsInBin ...)

		for j := 0; j < len(compoundsInBin); j++ {
			compoundBinIndices = append(compoundBinIndices, binIndices[i])
		}
	}

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

		if len(infos) < 3 {
			log.Printf("Failed to load infos from file %s.", fingerprints[fingerprintId].InfosFile)
			log.Printf("Line loaded: %s.", info)
			return BinResponseMessage{
				Command: 	"load:bin",
				Index:   	data[3],
				BinSize: 	"0",
			}
		}

		ids[i] = infos[0]
		smiles[i] = infos[1]
		fps[i] = infos[2]
		coords[i] = infos[2]

		if err != nil {
			log.Printf("Error loading bin: %v", err)
		}
	}

	return BinResponseMessage{
		Command: 	"load:bin",
		Smiles:  	smiles,
		Ids:     	ids,
		Coords:  	coords,
		Fps:     	fps,
		BinIndices: compoundBinIndices,
		Index:   	data[3],
		BinSize: 	strconv.Itoa(len(compounds)),
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
		log.Printf("Error while searching: %v", err)

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

func (c *Client) read() {
	defer c.conn.Close()

	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		msg := RequestMessage{}
		err := c.conn.ReadJSON(&msg)

		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				log.Printf("Error during reading: %v", err)
			}
			break
		}

		select {
		case c.send <- msg:
		default:
			close(c.send)
		}
	}
}

func (c *Client) write() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			var err error

			switch message.Command {
			case "init":
				err = c.conn.WriteJSON(underdarkInit(message.Content))
			case "load:variant":
				err = c.conn.WriteJSON(underdarkLoadVariant(message.Content))
			case "load:stats":
				err = c.conn.WriteJSON(underdarkLoadStats(message.Content))
			case "load:map":
				err = c.conn.WriteJSON(underdarkLoadMap(message.Content))
			case "load:binpreview":
				err = c.conn.WriteJSON(underdarkLoadBinPreview(message.Content))
			case "load:bin":
				err = c.conn.WriteJSON(underdarkLoadBin(message.Content))
			case "search:infos":
				err = c.conn.WriteJSON(underdarkSearch(message.Content))
			}

			if err != nil {
				log.Printf("Error during writing: %v", err)
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}

func serveUnderdark(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)

	if err != nil {
		log.Printf("Error while upgrading connection: %v", err)
		return
	}

	client := &Client{conn: conn, send: make(chan RequestMessage, 256)}
	go client.write()
	client.read()
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Example: " + os.Args[0] + " <data-path>")
		os.Exit(1)
	}

	if os.Getenv("DEBUG") == "TRUE" {
		fmt.Println("Debug mode")
		debug = true
	} else {
		debug = false
	}

	dataDir = os.Args[1]
	config = loadConfig()

	checkConfig()
	loadIndices()

	http.Handle("/", http.FileServer(http.Dir("./assets")))
	http.HandleFunc("/underdark", serveUnderdark)

	log.Println("Serving at localhost:8081 ...")
	log.Fatal(http.ListenAndServe(":8081", nil))
}

func loadIndices() {
	loopConfig(func(database *Database, path string) {
		// Nothing to do here

	}, func(fingerprint *Fingerprint, path string) {
		// Loading info indices and lengths
		infosLength, _ := countLines(fingerprint.InfoIndicesFile)

		infoOffsets[fingerprint.Id] = make([]uint64, infosLength)
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

func readIndexFile(path string, offsets []uint64, lengths []uint32) error {
	r, err := os.Open(path)
	scanner := bufio.NewScanner(r)

	i := 0
	for scanner.Scan() {
		line := scanner.Text()
		values := strings.Split(line, ",")

		offset, _ := strconv.ParseUint(values[0], 10, 64)
		length, _ := strconv.ParseUint(values[1], 10, 16)

		offsets[i] = uint64(offset)
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

	// Load the stats for this variant
	stats[id] = calcStats(id)

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
			sp := strings.Split(val, " ")
			if sp[0] == terms[j] {
				results[j] = append(results[j], uint32(i))
			} else if sp[1] == terms[j] {
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

func calcStats(variantId string) Stats {
	nBins := len(variantIndices[variantId])
	nCompounds := 0
	max := 0
	min := 9999

	for i := 0; i < nBins; i++ {
		n := len(variantIndices[variantId][i])
		nCompounds += n

		if n > max {
			max = n
		}

		if n < min {
			min = n
		}
	}

	var hist = make([]uint32, max+1)

	for i := 0; i < nBins; i++ {
		n := len(variantIndices[variantId][i])
		hist[n]++
	}

	return Stats{
		CompoundCount: uint32(nCompounds),
		BinCount:      uint32(nBins),
		AvgBinSize:    float32(nCompounds / nBins),
		BinHist:       hist,
		HistMin:       uint32(min),
		HistMax:       uint32(max),
	}
}

func stringToIntArray(arr []string) []uint32 {
	var result = []uint32{}

	for _, i := range arr {
		j, err := strconv.Atoi(i)

		if err != nil {
			log.Println(err)
		}

		result = append(result, uint32(j))
	}

	return result
}
