# Default values for delays
STARTUP_DELAY_MS=$1
LOOP_DELAY_MS=$2

trap 'echo "Received SIGTERM, shutting down..."; exit 0' SIGTERM

# Convert milliseconds to seconds
STARTUP_DELAY=$(awk "BEGIN {printf \"%.3f\", $STARTUP_DELAY_MS / 1000}")
LOOP_DELAY=$(awk "BEGIN {printf \"%.3f\", $LOOP_DELAY_MS / 1000}")

# Ensure parameters are passed
if [[ $# -ne 2 ]]; then
    echo "Usage: $0 <startup_delay_ms> <loop_delay_ms>"
    exit 1
fi


sleep "$STARTUP_DELAY"
echo 'Started'

while true; do
    echo "Service is running... $(date)"
    sleep "$LOOP_DELAY"
done
