# Проверяем, установлен ли Python
if (!(Get-Command python -ErrorAction SilentlyContinue)) {
    Write-Host "Error: Python is required. Please install it from https://www.python.org/downloads/"
    exit 1
}

# Проверяем наличие config.json
if (!(Test-Path "configs/config.json")) {
    Write-Host "Error: configs/config.json not found"
    exit 1
}

# Генерируем docker-compose.yml
Write-Host "Generating docker-compose.yml..."
python generate_docker_compose.py
if ($LASTEXITCODE -ne 0) {
    Write-Host "Error: Failed to generate docker-compose.yml"
    exit 1
}

# Проверяем, что docker-compose.yml создан
if (!(Test-Path "docker-compose.yml")) {
    Write-Host "Error: docker-compose.yml was not created"
    exit 1
}

# Запускаем Docker Compose
Write-Host "Stopping existing containers..."
docker-compose down
Write-Host "Starting containers..."
docker-compose up --build
if ($LASTEXITCODE -ne 0) {
    Write-Host "Error: Failed to start Docker Compose"
    exit 1
}