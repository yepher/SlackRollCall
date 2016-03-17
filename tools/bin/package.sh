#!/bin/bash

# This script make a lot of assumptions and has no error handling


BIN_DIR=`dirname "$0"`
cd $BIN_DIR/../..

BASE_DIR=`pwd`

echo "Base Dir: " $BASE_DIR

rm -rf $BASE_DIR/package
mkdir $BASE_DIR/package

cp Usage.md $BASE_DIR/package/Usage.md

###################
# Build and Package CLI for OSX
###################
echo "Building for OSX"

mkdir $BASE_DIR/package/osx
export GOOS="darwin"


cd $BASE_DIR
rm -f SlackRollCall
go build
mv SlackRollCall $BASE_DIR/package/osx


###################
# Build and Package CLI for Linux
###################


echo "Building for Linux"

mkdir $BASE_DIR/package/linux
export GOOS="linux"

cd $BASE_DIR
rm -f SlackRollCall
go build
mv SlackRollCall $BASE_DIR/package/linux

# Build and Package CLI for windows
###################


echo "Building for Windows"

mkdir $BASE_DIR/package/windows
export GOOS="windows"

cd $BASE_DIR
rm -f SlackRollCall.exe
go build
mv SlackRollCall.exe $BASE_DIR/package/windows


###################
# Done!!!
###################
export GOOS=""

echo "Done..."
echo ""
echo "See $BASE_DIR/package for binary files"
open $BASE_DIR/package
