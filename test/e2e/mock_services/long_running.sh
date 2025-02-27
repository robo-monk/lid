#!/bin/bash
# This script simulates a long-running service that responds to signals

trap 'echo "Received SIGTERM, shutting down..."; exit 0' SIGTERM
trap 'echo "Received SIGINT, shutting down..."; exit 0' SIGINT

sleep .2
echo "Starting long-running service..."

while true; do
    echo "Service is running... $(date)"
    sleep 0.1
done
