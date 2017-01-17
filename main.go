package main

import (
	"encoding/json"
	"fmt"
	"github.com/googollee/go-socket.io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
)

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
var variantIndices map[string][][]int

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Example: " + os.Args[0] + " <data-path>")
		os.Exit(1)
	}

	dataDir = os.Args[1]
	config = loadConfig()

	checkConfig()
	loadIndices()

	server, err := socketio.NewServer(nil)
	if err != nil {
		log.Fatal(err)
	}

	server.On("connection", func(so socketio.Socket) {
		log.Println("on connection")
	})

	http.Handle("/socket.io/", server)
	http.Handle("/", http.FileServer(http.Dir("./asset")))

	log.Println("Serving at localhost:8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
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
	})

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
	colorMapCallback func(*ColorMap, string)) {
	for i, _ := range config.Databases {
		database := &config.Databases[i]
		databasePath := concatPath(dataDir, database.Directory)
		databaseCallback(database, databasePath)
		for j, _ := range database.Fingerprints {
			fingerprint := &database.Fingerprints[j]
			fingerprintPath := concatPath(databasePath, fingerprint.Directory)
			fingerprintCallback(fingerprint, fingerprintPath)
			for k, _ := range fingerprint.Variants {
				variant := &fingerprint.Variants[k]
				variantPath := concatPath(fingerprintPath, variant.Directory)
				variantCallback(variant, variantPath)
				for l, _ := range variant.ColorMaps {
					colorMap := &variant.ColorMaps[l]
					colorMapCallback(colorMap, variantPath)
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

func loadIndices() {

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