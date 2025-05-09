import json
import yaml
import socket
import sys
from pathlib import Path

def load_config(config_path):
    """Load and validate config.json."""
    config_file = Path(config_path)
    if not config_file.exists():
        print(f"Error: Config file {config_path} not found")
        sys.exit(1)
    with open(config_file, 'r', encoding='utf-8') as f:
        config = json.load(f)
    print(f"Loaded config with {len(config.get('backends', []))} backends: {config.get('backends', [])}")
    return config

def ensure_html_file(idx):
    """Create HTML file for backend if it doesn't exist."""
    html_path = Path(f'configs/index-backend{idx}.html')
    html_path.parent.mkdir(exist_ok=True)
    if not html_path.exists():
        with open(html_path, 'w', encoding='utf-8') as f:
            f.write(f'<!DOCTYPE html><html><head><title>Welcome to Nginx!</title></head><body><h1>Hello from Nginx Backend {idx}!</h1></body></html>')
        print(f"Created HTML file: {html_path}")
    else:
        print(f"HTML file already exists: {html_path}")

def find_free_port(start_port):
    """Find an available port starting from start_port."""
    port = start_port
    max_attempts = 100
    attempt = 0
    while attempt < max_attempts:
        with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
            try:
                s.bind(('localhost', port))
                print(f"Assigned port {port} for backend")
                return port
            except OSError as e:
                print(f"Port {port} is in use: {e}")
                port += 1
                attempt += 1
    print(f"Error: Could not find a free port after {max_attempts} attempts")
    sys.exit(1)

def generate_docker_compose(config):
    """Generate docker-compose.yml based on config.json."""
    backends = config.get('backends', [])
    base_port = 8001

    compose_config = {
        'version': '3.8',
        'services': {
            'load-balancer': {
                'container_name': 'load-balancer',
                'build': {
                    'context': '.',
                    'dockerfile': 'Dockerfile'
                },
                'ports': ['8087:8087'],
                'volumes': ['./configs:/app/configs'],
                'depends_on': {},
                'environment': ['LOG_LEVEL=DEBUG'],
                'networks': ['balancer-net']
            },
            'redis': {
                'image': 'redis:7.4',
                'ports': ['6379:6379'],
                'volumes': ['redis-data:/data'],
                'healthcheck': {
                    'test': ['CMD', 'redis-cli', 'ping'],
                    'interval': '5s',
                    'timeout': '3s',
                    'retries': 5
                },
                'networks': ['balancer-net']
            }
        },
        'networks': {
            'balancer-net': {'driver': 'bridge'}
        },
        'volumes': {
            'redis-data': {}
        }
    }

    for idx, backend_url in enumerate(backends, 1):
        backend_name = f'backend{idx}'
        print(f"Processing backend {idx}: {backend_url}")
        ensure_html_file(idx)
        host_port = find_free_port(base_port + idx - 1)
        compose_config['services'][backend_name] = {
            'image': 'nginx:1.27',
            'container_name': backend_name,
            'ports': [f'{host_port}:80'],
            'volumes': [
                './configs/nginx.conf:/etc/nginx/nginx.conf',
                f'./configs/index-backend{idx}.html:/usr/share/nginx/html/index.html'
            ],
            'healthcheck': {
                'test': ['CMD', 'curl', '-f', 'http://localhost/health'],
                'interval': '5s',
                'timeout': '3s',
                'retries': 5
            },
            'networks': ['balancer-net']
        }
        compose_config['services']['load-balancer']['depends_on'][backend_name] = {
            'condition': 'service_healthy'
        }
        print(f"Added service {backend_name} with port {host_port}")

    with open('docker-compose.yml', 'w', encoding='utf-8') as f:
        yaml.dump(compose_config, f, default_flow_style=False, sort_keys=False)
    print("Generated docker-compose.yml successfully")

if __name__ == '__main__':
    config_path = 'configs/config.json'
    config = load_config(config_path)
    generate_docker_compose(config)