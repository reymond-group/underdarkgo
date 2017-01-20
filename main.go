package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"io/ioutil"
	"log"
	"net/http"
	"os"
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
	Id                    string    `json:"id"`
	Name                  string    `json:"name"`
	Description           string    `json:"description"`
	Directory             string    `json:"directory"`
	CoordinatesFile       string    `json:"coordinatesFile"`
	CoordinateIndicesFile string    `json:"coordinateIndicesFile"`
	Variants              []Variant `json:"variants"`
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

var variantIndices map[string][][]uint32
var smilesIndices map[string][]uint32
var smilesLengths map[string][]uint16
var idIndices map[string][]uint32
var idLengths map[string][]uint16

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func underdarkInit(data []string) InitResponseMessage {
	return InitResponseMessage{
		Command: "cmd",
		Content: config,
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

		if msg.Command == "init" {
			err = c.WriteJSON(underdarkInit(msg.Content))
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

	http.Handle("/", http.FileServer(http.Dir("./asset")))
	http.HandleFunc("/underdark", serveUnderdark)

	log.Println("Serving at localhost:8080 ...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func loadIndices() {
	// Initialize the maps holding the indices
	variantIndices := make(map[string][][]uint32)
	smilesIndices := make(map[string][]uint32)
	smilesLengths := make(map[string][]uint16)
	idIndices := make(map[string][]uint32)
	idLengths := make(map[string][]uint16)

	loopConfig(func(database *Database, path string) {
		// Loading smiles and id indices and lengths
		smilesIndices[database.Id] = make([]uint32, 10000000)
		smilesLengths[database.Id] = make([]uint16, 10000000)
		idIndices[database.Id] = make([]uint32, 10000000)
		idLengths[database.Id] = make([]uint16, 10000000)

		for i := 0; i < 10000000; i++ {
			smilesIndices[database.Id][i] = 100
			smilesLengths[database.Id][i] = 100
			idIndices[database.Id][i] = 100
			idLengths[database.Id][i] = 100
		}
	}, func(fingerprint *Fingerprint, path string) {
		// Loading indices for coordinates

	}, func(variant *Variant, path string) {
		// Loading the bin contents (indices pointing to the
		// smiles and ids
		variantIndices[variant.Id] = make([][]uint32, 2)
		variantIndices[variant.Id][0] = make([]uint32, 10000000)
		variantIndices[variant.Id][1] = make([]uint32, 10000000)

	}, func(colorMap *ColorMap, path string) {
		// Nothing to do here
	}, false)
}

func loadConfig() Configuration {
	buffer, err := ioutil.ReadFile("config.json")

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
	}, func(fingerprint *Fingerprint, path string) {
		fingerprint.CoordinatesFile = path + fingerprint.CoordinatesFile
		fingerprint.CoordinateIndicesFile = path + fingerprint.CoordinateIndicesFile

		if exists, _ := exists(fingerprint.CoordinatesFile); !exists {
			nf = append(nf, fingerprint.CoordinatesFile)
		}

		if exists, _ := exists(fingerprint.CoordinateIndicesFile); !exists {
			nf = append(nf, fingerprint.CoordinateIndicesFile)
		}
	}, func(variant *Variant, path string) {
		variant.IndicesFile = path + variant.IndicesFile
		variant.CoordinatesFile = path + variant.CoordinatesFile

		if exists, _ := exists(variant.IndicesFile); !exists {
			nf = append(nf, variant.IndicesFile)
		}

		if exists, _ := exists(variant.CoordinatesFile); !exists {
			nf = append(nf, variant.CoordinatesFile)
		}
	}, func(colorMap *ColorMap, path string) {
		colorMap.MapFile = path + colorMap.MapFile

		if exists, _ := exists(colorMap.MapFile); !exists {
			nf = append(nf, colorMap.MapFile)
		}
	}, true)

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
	updatePath bool) {
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
			if updatePath {
				fingerprintPath = concatPath(databasePath, fingerprint.Directory)
				fingerprintCallback(fingerprint, fingerprintPath)
			} else {
				fingerprintCallback(fingerprint, fingerprint.Directory)
			}
			for k, _ := range fingerprint.Variants {
				variant := &fingerprint.Variants[k]
				var variantPath string
				if updatePath {
					variantPath = concatPath(fingerprintPath, variant.Directory)
					variantCallback(variant, variantPath)
				} else {
					variantCallback(variant, variant.Directory)
				}
				for l, _ := range variant.ColorMaps {
					colorMap := &variant.ColorMaps[l]
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
