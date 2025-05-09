package balancer

import (
	"sync"

	"load-balancer/internal/models"
)

// BalancerInterface определяет методы для балансировщика.
type BalancerInterface interface {
	NextBackend() *models.Backend
}

// Balancer управляет списком бэкендов и выбирает следующий доступный.
type Balancer struct {
	backends []*models.Backend
	current  int
	mu       sync.Mutex
}

// NewBalancer создает новый экземпляр балансировщика.
func NewBalancer(backends []*models.Backend) *Balancer {
	return &Balancer{
		backends: backends,
		current:  -1, // Начинаем с -1, чтобы первый вызов выбрал backends[0]
	}
}

// NextBackend возвращает следующий доступный бэкенд по алгоритму round-robin.
func (b *Balancer) NextBackend() *models.Backend {
	b.mu.Lock()
	defer b.mu.Unlock()

	for i := 0; i < len(b.backends); i++ {
		b.current = (b.current + 1) % len(b.backends)
		backend := b.backends[b.current]
		if backend.Healthy {
			return backend
		}
	}
	return nil
}

// ResetCurrent сбрасывает текущий индекс для тестов.
func (b *Balancer) ResetCurrent() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.current = -1
}
