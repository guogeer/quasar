#!/bin/bash

if [ "$1" = "-install" ]; then
	echo "test"
	go get -d \
	    github.com/guogeer/husky/...
	go get -u \
	    github.com/guogeer/husky/{gateway,router}
fi
