#!/bin/bash

if [[ ! -z $(find internal/gen/ -type f -name '*.pb.go') ]]; then
	rm $(find internal/gen/ -type f -name '*.pb.go')
fi
