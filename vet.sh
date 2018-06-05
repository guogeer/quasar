#!/bin/bash

if [ "$1" = "-install" ]; then
	go get -d \
	    github.com/guogeer/husky/...
	go get -u \
	    github.com/guogeer/husky/{gateway,router}
fi
