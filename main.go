package main

import (
    "io/ioutil"
    "encoding/json"
    "log"
    "net/http"
    "github.com/googollee/go-socket.io"
)

type Map struct {
    Id          string `json:"id"`
    Name        string `json:"name"`
    Description string `json:"description"`
    MapFile     string `json:"mapFile"`
    DataTypes   []string `json:"dataTypes"`
}

type Variant struct {
    Id              string `json:"id"`
    Name            string `json:"name"`
    Description     string `json:"description"`
    Resolution      int `json:"resolution"`
    DataTypes       []string `json:"dataTypes"`
    Directory       string `json:"directory"`
    IndicesFile     string `json:"indicesFile"`
    CoordinatesFile string `json:"coordinatesFile"`
    Maps            []Map `json:"maps"`
}

type Fingerprint struct {
    Id                    string `json:"id"`
    Name                  string `json:"name"`
    Description           string `json:"description"`
    Directory             string `json:"directory"`
    CoordinatesFile       string `json:"coordinatesFile"`
    CoordinateIndicesFile string `json:"coordinateIndicesFile"`
    Variants              []Variant `json:"variants"`
}

type Database struct {
    Id                string `json:"id"`
    Name              string `json:"name"`
    Description       string `json:"description"`
    Directory         string `json:"directory"`
    SmilesFile        string `json:"smilesFile"`
    SmilesIndicesFile string `json:"smilesIndicesFile"`
    IdsFile           string `json:"idsFile"`
    IdIndicesFile     string `json:"idIndicesFile"`
    Fingerprints      []Fingerprint `json:"fingerprints"`
}

type Configuration struct {
    Databases []Database `json:"databases"`
}

func main() {
    loadConfig()

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

func loadConfig() {
    buffer, err := ioutil.ReadFile("config.json")
    
    if err != nil {
        log.Fatal(err)
    }
    
    var config Configuration
    err = json.Unmarshal(buffer, &config)
    
    if err != nil {
        log.Fatal(err)
    }

    log.Println(config.Databases[0].Fingerprints[0].Id)
}
