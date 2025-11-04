#!/bin/sh
cd $(dirname $0)
go build -C cmd/iron -o ../../iron
./iron version
