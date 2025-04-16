#!/bin/bash

echo "Waiting for Kafka to be ready..."
sleep 7

for i in {1..10}; do
    echo "Attempt $i to connect to Kafka..."
    if kafka-topics --bootstrap-server kafka:29092 --list > /dev/null 2>&1; then
        echo "Kafka is ready!"
        break
    fi
    if [ $i -eq 10 ]; then
        echo "Kafka connection failed after 10 attempts"
        exit 1
    fi
    sleep 10
done

echo "Creating topics..."
kafka-topics --create --if-not-exists --topic product-events --bootstrap-server kafka:29092 --partitions 1 --replication-factor 1
kafka-topics --create --if-not-exists --topic product-commands --bootstrap-server kafka:29092 --partitions 1 --replication-factor 1
echo "Topics created successfully!"