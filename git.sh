#!/bin/bash

a=$(date +%A%d%B)

git add .
git commit -m $a
git push origin dong/t10