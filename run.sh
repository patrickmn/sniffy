#!/bin/sh
set -u
set -e
cd suite
go install
GOMAXPROCS=4 suite $@
