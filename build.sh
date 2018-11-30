#!/usr/bin/env bash

go build main.go
mv main bin/ytblock
env GOOS=linux GOARCH=arm GOARM=5 go build main.go
mv main bin/ytblock-rpi