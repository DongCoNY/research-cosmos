#!/bin/bash

injectived rosetta \
  --addr ":8080" \
  --blockchain "injective" \
  --grpc "localhost:9900" \
  --network "injective-1" \
  --tendermint "localhost:26657" \
  --log-level "debug" \
  --trace
