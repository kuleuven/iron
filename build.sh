#!/bin/sh
cd $(dirname $0)
CGO_ENABLED=0 go build -C cmd/iron -o ../../iron
./iron version
