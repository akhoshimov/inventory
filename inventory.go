package main

import (
	"errors"
	"sync"
)

var (
	ErrProductNotFound   = errors.New("product not found")
	ErrInsufficientStock = errors.New("insufficient stock")
)

type ReserveItem struct {
	ProductID string
	Quantity  int
}

type Product struct {
	ID    string
	Name  string
	Stock int
}

type InventoryService struct {
	products map[string]*Product
	mu       sync.RWMutex
}

func (s *InventoryService) GetStock(productID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	product := s.products[productID]
	if product == nil {
		return 0
	}
	return product.Stock
}

func (s *InventoryService) Reserve(productID string, quantity int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	product := s.products[productID]
	if product == nil {
		return ErrProductNotFound
	}

	if product.Stock < quantity {
		return ErrInsufficientStock
	}

	product.Stock -= quantity
	return nil
}

func (s *InventoryService) ReserveMultiple(items []ReserveItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check all first
	for _, item := range items {
		product := s.products[item.ProductID]
		if product == nil {
			return ErrProductNotFound
		}
		if product.Stock < item.Quantity {
			return ErrInsufficientStock
		}
	}

	// Then reserve all
	for _, item := range items {
		s.products[item.ProductID].Stock -= item.Quantity
	}

	return nil
}

func (s *InventoryService) SafeReserve(productID string, quantity int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	product := s.products[productID]
	if product.Stock < quantity {
		return ErrInsufficientStock
	}

	product.Stock -= quantity
	return nil
}
