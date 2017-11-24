![Underdark Go](https://github.com/reymond-group/underdarkgo/blob/master/logo.png?raw=true)

If you use this code or application, please cite the original paper published by Bioinformatics: ![10.1093/bioinformatics/btx760](http://dx.doi.org/10.1093/bioinformatics/btx760)
# Underdark Go

**For a complete overview and detailed installation instructions for this project, please visit the [project website](http://doc.gdb.tools/fun).**

## Getting Started
The easiest way to get started with Underdark Go is pulling and running the docker image.
```
docker run -d -p 80:8081 -v /your/host/dir:/underdarkgo/data --name underdark daenuprobst/underdark-go
```
Underdark Go exposes port `8081` by default. Depending on your network settings and topology you might want to change this using the `-p` argument (see example above). The directory `/underdarkgo/data` has to be mounted to a host directory (`/your/host/dir` in the example above) where a configuration file `config.json` describes the data within a sub-directory.
## Configuration
The file `config.json` stores four levels of meta-data on the data to be provided via the service.
1. Database information
2. Fingerprint information
3. Variant information
4. Map information

These levels also reflect the directory tree in which the files containing the data are stored. The configuration file `config.json` has the following format
```json
{
    "databases": [
        {
            "id": "acmebase-2",
            "name": "ACMEbase Version 2.0",
            "description": "A database that contains everything.",
            "directory": "acmebase2",
            "fingerprints": [
                {
                    "id": "xfp",
                    "name": "Xfp",
                    "description": "The Xfp fingerprint.",
                    "directory": "Xfp",
                    "infosFile": "acmebase2.xfp.info",
                    "infoIndicesFile": "acmebase2.xfp.info.index",
                    "variants": [
                        {
                            "id": "250",
                            "name": "Low Resolution",
                            "description": "Binned with a resolution of 250 x 250 x 250.",
                            "resolution": 250,
                            "dataTypes": [
                                "uint16",
                                "uint16",
                                "uint16"
                            ],
                            "directory": "250",
                            "indicesFile": "acmebase2.xfp.250.dat",
                            "coordinatesFile": "acmebase2.xfp.250.xyz",
                            "maps": [
                                {
                                    "id": "hac",
                                    "name": "Heavy Atom Count",
                                    "description": "The number of heavy (non-hydrogen) atoms in the molecule.",
                                    "mapFile": "acmebase2.xfp.250.1.map",
                                    "dataTypes": [
                                        "float32",
                                        "float32",
                                        "float32"
                                    ]
                                },
                                ...
                            ]
                        },
                        ...
                    ]
                },
                ...
            ]
        }
    ]
}
```
The directory structure of the above example would thus look like this
```
.
+-- config.json
+-- acmebase2
    +-- Xfp
        |-- acmebase2.xfp.info
        |-- acmebase2.xfp.info.index
        +-- 250
            |-- acmebase2.xfp.250.1.map
```
All files can be generated from initial files containing one molecular fingerprint (of any type) per line. Python 3.x scripts as well as a bash script for automation can be found [here](https://github.com/reymond-group/pca). This repository also contains a dockerized flask based project to enable the PCA projection of additional molecular fingerprints using the models generated for the initial data set.
## Build
[Download Go](https://golang.org/dl/)

Install Go
```bash
sudo tar -xvf gox.y.linux-amd64.tar.gz
sudo mv go /usr/local
```
Create a directory where built Go executables will be stored
```bash
mkdir ~/go
mkdir ~/go/bin
```
Edit `~/.profile` by adding (at the end)
```bash
export PATH=$PATH:/usr/local/go/bin
export GOPATH=/usr/local/go/bin
export GOBIN=$HOME/go/bin
# GOBIN is set to the directory created in the previous step
```
Update profile
```bash
source ~/.profile
```
Get the socket.io package
```bash
go get github.com/gorilla/websocket
```
You can the build the project
```bash
go build -o underdarkgo main.go
```
