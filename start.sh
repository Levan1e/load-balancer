#!/bin/bash

# Проверяем, установлен ли Python
if ! command -v python3 &> /dev/null; then
    echo "Error: Python3 is required. Please install it (e.g., 'sudo apt install python3' on Ubuntu)."
    exit 1
fi

# Проверяем наличие config.json
if [ ! -f "configs/config.json" ]; then
    echo "Error: configs/config.json not found"
    exit 1
fi

# Генерируем docker-compose.yml
echo "Generating docker-compose.yml..."
python3 generate_docker_compose.py
if [ $? -ne 0 ]; then
    echo "Error: Failed to generate docker-compose.yml"
    exit 1
fi

# Проверяем, что docker-compose.yml создан
if [ ! -f "docker-compose.yml" ]; then
    echo "Error: docker-compose.yml was not created"
    exit 1
fi

# Запускаем Docker Compose
echo "Stopping existing containers..."
docker-compose down
echo "Starting containers..."
docker-compose up --build
if [ $? -ne 0 ]; then
    echo "Error: Failed to start Docker Compose"
    exit 1
fi