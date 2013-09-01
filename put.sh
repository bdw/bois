#!/bin/bash

FILE=example.jpg

curl --verbose -H "content-type: image/jpeg" --data-binary @$FILE -X "PUT" \
    http://localhost:8080/image
