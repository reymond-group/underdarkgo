# underdarkgo

## Build
Download go
```
https://golang.org/dl/
```
Install go
```
sudo tar -xvf gox.y.linux-amd64.tar.gz
sudo mv go /usr/local
```
Create a directory where built go executables will be stored
```
mkdir ~/go
mkdir ~/go/bin
```
Edit `~/.profile` by adding (at the end)
```
export PATH=$PATH:/usr/local/go/bin
export GOBIN=$HOME/go/bin
# GOBIN is set to the directory created in the previous step
```
Update profile
```
source ~/.profile
```
