#!/bin/bash
# This script simulates a service that crashes randomly

echo "Starting crash-prone service..."
COUNTER=0

while true; do
    COUNTER=$((COUNTER + 1))
    echo "Service iteration $COUNTER"

    # Randomly exit with error ~20% of the time
    if [ $((RANDOM % 5)) -eq 0 ]; then
        echo "Simulating crash..."
        exit 1
    fi

    sleep 0.01
done
