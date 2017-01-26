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
	Id                     string    `json:"id"`
	Name                   string    `json:"name"`
	Description            string    `json:"description"`
	Directory              string    `json:"directory"`
	CoordinatesFile        string    `json:"coordinatesFile"`
	CoordinateIndicesFile  string    `json:"coordinateIndicesFile"`
	FingerprintsFile       string    `json:"fingerprintsFile"`
	FingerprintIndicesFile string    `json:"fingerprintIndicesFile"`
	Variants               []Variant `json:"variants"`
}

type Database struct {
	Id                string        `json:"id"`
	Name              string        `json:"name"`
	Description       string        `json:"description"`
	Directory         string        `json:"directory"`
	SmilesFile        string        `json:"smilesFile"`
	SmilesIndicesFile string        `json:"smilesIndicesFile"`
	IdsFile           string        `json:"idsFile"`
	IdIndicesFile     string        `json:"idIndicesFile"`
	Fingerprints      []Fingerprint `json:"fingerprints"`
}

type Configuration struct {
	Databases []Database `json:"databases"`
}

var dataDir string
var config Configuration

var variantIndices = map[string][][]uint32{}
var smilesOffsets = map[string][]uint32{}
var smilesLengths = map[string][]uint16{}
var idOffsets = map[string][]uint32{}
var idLengths = map[string][]uint16{}
var coordOffsets = map[string][]uint32{}
var coordLengths = map[string][]uint16{}
var fpOffsets = map[string][]uint32{}
var fpLengths = map[string][]uint16{}

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
	databaseId := data[0]
	variantId := data[1]
	binIndex, _ := strconv.Atoi(data[2])

	file, err := os.Open(databases[databaseId].SmilesFile)

	if err != nil {
		log.Fatal(err)
	}

	defer file.Close()

	// Get the indices in the bin
	compounds := variantIndices[variantId][binIndex]

	smilesOffset := smilesOffsets[databaseId][compounds[0]]
	smilesLength := smilesLengths[databaseId][compounds[0]]

	buf := make([]byte, int64(smilesLength))
	rn, err := file.ReadAt(buf, int64(smilesOffset))

	return BinPreviewResponseMessage{
		Command: "load:binpreview",
		Smiles:  string(buf[:rn-1]),
		Index:   data[2],
		BinSize: strconv.Itoa(len(compounds)),
	}
}

func underdarkLoadBin(data []string) BinResponseMessage {
	databaseId := data[0]
	fingerprintId := data[1]
	variantId := data[2]
	binIndex, _ := strconv.Atoi(data[3])

	smilesFile, err := os.Open(databases[databaseId].SmilesFile)
	idsFile, err := os.Open(databases[databaseId].IdsFile)
	coordsFile, err := os.Open(fingerprints[fingerprintId].CoordinatesFile)
	fpsFile, err := os.Open(fingerprints[fingerprintId].FingerprintsFile)

	if err != nil {
		log.Fatal(err)
	}

	defer smilesFile.Close()
	defer idsFile.Close()
	defer coordsFile.Close()

	// Get the indices in the bin
	compounds := variantIndices[variantId][binIndex]

	length := len(compounds)
	smiles := make([]string, length)
	ids := make([]string, length)
	coords := make([]string, length)
	fps := make([]string, length)

	for i := 0; i < length; i++ {
		smilesOffset := smilesOffsets[databaseId][compounds[i]]
		smilesLength := smilesLengths[databaseId][compounds[i]]

		idOffset := idOffsets[databaseId][compounds[i]]
		idLength := idLengths[databaseId][compounds[i]]

		coordOffset := coordOffsets[fingerprintId][compounds[i]]
		coordLength := coordLengths[fingerprintId][compounds[i]]

		fpOffset := fpOffsets[fingerprintId][compounds[i]]
		fpLength := fpLengths[fingerprintId][compounds[i]]

		buf := make([]byte, int64(smilesLength))
		rn, err := smilesFile.ReadAt(buf, int64(smilesOffset))
		smiles[i] = string(buf[:rn-1])

		buf = make([]byte, int64(idLength))
		rn, err = idsFile.ReadAt(buf, int64(idOffset))
		ids[i] = string(buf[:rn-1])

		buf = make([]byte, int64(coordLength))
		rn, err = coordsFile.ReadAt(buf, int64(coordOffset))
		coords[i] = string(buf[:rn-1])

		buf = make([]byte, int64(fpLength))
		rn, err = fpsFile.ReadAt(buf, int64(fpOffset))
		fps[i] = string(buf[:rn-1])

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
		// Loading smiles and id indices and lengths
		// Smiles and id indices should have the same length
		smilesLength, _ := countLines(database.SmilesIndicesFile)
		idsLength, _ := countLines(database.IdIndicesFile)

		smilesOffsets[database.Id] = make([]uint32, smilesLength)
		smilesLengths[database.Id] = make([]uint16, smilesLength)
		idOffsets[database.Id] = make([]uint32, idsLength)
		idLengths[database.Id] = make([]uint16, idsLength)

		log.Println("Reading " + database.SmilesIndicesFile + " ...")

		err := readIndexFile(database.SmilesIndicesFile, smilesOffsets[database.Id], smilesLengths[database.Id])

		if err != nil {
			log.Fatal(err)
		}

		log.Println("Reading " + database.IdIndicesFile + " ...")

		err = readIndexFile(database.IdIndicesFile, idOffsets[database.Id], idLengths[database.Id])

		if err != nil {
			log.Fatal(err)
		}

	}, func(fingerprint *Fingerprint, path string) {
		// Loading indices for coordinates and fingerprints
		coordsLength, _ := countLines(fingerprint.CoordinateIndicesFile)
		fpsLength, _ := countLines(fingerprint.FingerprintIndicesFile)

		coordOffsets[fingerprint.Id] = make([]uint32, coordsLength)
		coordLengths[fingerprint.Id] = make([]uint16, coordsLength)
		fpOffsets[fingerprint.Id] = make([]uint32, fpsLength)
		fpLengths[fingerprint.Id] = make([]uint16, fpsLength)

		log.Println("Reading " + fingerprint.CoordinateIndicesFile + " ...")

		err := readIndexFile(fingerprint.CoordinateIndicesFile, coordOffsets[fingerprint.Id], coordLengths[fingerprint.Id])

		if err != nil {
			log.Fatal(err)
		}

		log.Println("Reading " + fingerprint.FingerprintIndicesFile + " ...")

		err = readIndexFile(fingerprint.FingerprintIndicesFile, fpOffsets[fingerprint.Id], fpLengths[fingerprint.Id])

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
		database.SmilesFile = path + database.SmilesFile
		database.SmilesIndicesFile = path + database.SmilesIndicesFile
		database.IdsFile = path + database.IdsFile
		database.IdIndicesFile = path + database.IdIndicesFile

		if exists, _ := exists(database.SmilesFile); !exists {
			nf = append(nf, database.SmilesFile)
		}

		if exists, _ := exists(database.SmilesIndicesFile); !exists {
			nf = append(nf, database.SmilesIndicesFile)
		}

		if exists, _ := exists(database.IdsFile); !exists {
			nf = append(nf, database.IdsFile)
		}

		if exists, _ := exists(database.IdIndicesFile); !exists {
			nf = append(nf, database.IdIndicesFile)
		}

		databases[database.Id] = *database

	}, func(fingerprint *Fingerprint, path string) {
		fingerprint.CoordinatesFile = path + fingerprint.CoordinatesFile
		fingerprint.CoordinateIndicesFile = path + fingerprint.CoordinateIndicesFile

		fingerprint.FingerprintsFile = path + fingerprint.FingerprintsFile
		fingerprint.FingerprintIndicesFile = path + fingerprint.FingerprintIndicesFile

		if exists, _ := exists(fingerprint.CoordinatesFile); !exists {
			nf = append(nf, fingerprint.CoordinatesFile)
		}

		if exists, _ := exists(fingerprint.CoordinateIndicesFile); !exists {
			nf = append(nf, fingerprint.CoordinateIndicesFile)
		}

		if exists, _ := exists(fingerprint.FingerprintsFile); !exists {
			nf = append(nf, fingerprint.FingerprintsFile)
		}

		if exists, _ := exists(fingerprint.FingerprintIndicesFile); !exists {
			nf = append(nf, fingerprint.FingerprintIndicesFile)
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

func readIndexFile(path string, offsets []uint32, lengths []uint16) error {
	r, err := os.Open(path)
	scanner := bufio.NewScanner(r)

	i := 0
	for scanner.Scan() {
		line := scanner.Text()
		values := strings.Split(line, ",")

		offset, _ := strconv.ParseUint(values[0], 10, 32)
		length, _ := strconv.ParseUint(values[1], 10, 16)

		offsets[i] = uint32(offset)
		lengths[i] = uint16(length)

		i++
	}

	return err
}

func readVariantIndexFile(path string, id string) error {
	r, err := os.Open(path)
	scanner := bufio.NewScanner(r)

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
