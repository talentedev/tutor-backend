#!/usr/bin/env bash
echo "Checking golangci-lint..."
golangci-lint run pkg

if [ $? -eq 1 ]; then
    exit 1
fi

EXIT_CODE=0

echo "Checking gofmt... format these files"

for file in `ls | grep -v vendor | grep -v clients | grep -v pkg | xargs -I {} find {} -name "*.go"|xargs -I {} gofmt -l {}`; do
	echo "go fmt $file"
	EXIT_CODE=1
done

if [ $EXIT_CODE == 1 ]; then
  exit 1
fi