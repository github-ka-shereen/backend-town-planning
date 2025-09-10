#!/bin/bash

echo "Starting looped build/run..."
while true; do
  go build -o tmp/main ./cmd && ./tmp/main
  echo "App crashed. Restarting in 2 seconds..."
  sleep 2
done
