#!/bin/bash

if [ "$1" = "-install" ]; then
	echo "test"
	go get -u \
	    github.com/guogeer/husky/...
fi
