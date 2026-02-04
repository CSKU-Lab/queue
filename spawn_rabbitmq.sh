#!/bin/bash

CONTAINER_NAME="rabbitmq_test_server"

# 1. Cleanup any existing container from a previous run
docker rm -f $CONTAINER_NAME > /dev/null 2>&1

# 2. Start RabbitMQ
echo "Starting RabbitMQ..."
docker run -d \
  --name $CONTAINER_NAME \
  --hostname queue \
  -p 5673:5672 \
  -p 15673:15672 \
  -e RABBITMQ_DEFAULT_USER=admin \
  -e RABBITMQ_DEFAULT_PASS=password \
  --health-cmd "rabbitmq-diagnostics -q ping" \
  --health-interval 2s \
  --health-retries 10 \
  rabbitmq:4.0-management > /dev/null

# 3. Wait for the service to be ready
echo -n "Waiting for RabbitMQ to be healthy..."
until [ "$(docker inspect -f '{{.State.Health.Status}}' $CONTAINER_NAME)" == "healthy" ]; do
    echo -n "."
    sleep 1
done

echo -e "\nRabbitMQ is UP! Running tests..."
