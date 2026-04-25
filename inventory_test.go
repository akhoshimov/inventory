package main

import (
	"errors"
	"sync"
	"testing"
)

func TestReserve_ConcurrentOversell(t *testing.T) {
	// Product has 100 stock
	// 200 goroutines each try to reserve 1
	// With race condition: some succeed even when they shouldn't
	// With proper locking: exactly 100 succeed, 100 fail

	service := &InventoryService{
		products: map[string]*Product{
			"1": {ID: "1", Name: "Product 1", Stock: 100},
		},
	}

	var (
		mu        sync.Mutex
		succeeded int
	)

	var wg sync.WaitGroup
	wg.Add(200)
	for i := 0; i < 200; i++ {
		go func() {
			defer wg.Done()
			err := service.Reserve("1", 1)
			if err == nil {
				mu.Lock()
				succeeded++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if succeeded != 100 {
		t.Errorf("expected exactly 100 to succeed, got %d", succeeded)
	}
	if service.GetStock("1") != 0 {
		t.Errorf("expected stock to be 0, got %d", service.GetStock("1"))
	}
}

func TestSafeReserve_ConcurrentOversell(t *testing.T) {
	// Product has 100 stock
	// 200 goroutines each try to reserve 1
	// With race condition: some succeed even when they shouldn't
	// With proper locking: exactly 100 succeed, 100 fail

	service := &InventoryService{
		products: map[string]*Product{
			"1": {ID: "1", Name: "Product 1", Stock: 100},
		},
	}

	var (
		mu        sync.Mutex
		succeeded int
	)

	var wg sync.WaitGroup
	wg.Add(200)
	for i := 0; i < 200; i++ {
		go func() {
			defer wg.Done()
			err := service.SafeReserve("1", 1)
			if err == nil {
				mu.Lock()
				succeeded++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if succeeded != 100 {
		t.Errorf("expected exactly 100 to succeed, got %d", succeeded)
	}
	if service.GetStock("1") != 0 {
		t.Errorf("expected stock to be 0, got %d", service.GetStock("1"))
	}
}

func TestReserveMultiple_Atomicity(t *testing.T) {
	// Product A: 10 stock, Product B: 5 stock
	// Try to reserve 8 of A and 8 of B
	// Should fail entirely (B doesn't have enough)
	// Both A and B should remain unchanged
	service := &InventoryService{
		products: map[string]*Product{
			"A": {ID: "A", Name: "Product A", Stock: 10},
			"B": {ID: "B", Name: "Product B", Stock: 5},
		},
	}

	err := service.ReserveMultiple([]ReserveItem{
		{ProductID: "A", Quantity: 8},
		{ProductID: "B", Quantity: 8},
	})
	if !errors.Is(err, ErrInsufficientStock) {
		t.Errorf("Expected ReserveMultiple to return ErrInsufficientStock, got %v", err)
	}

	if service.GetStock("A") != 10 {
		t.Errorf("Expected stock of A to be 10, got %d", service.GetStock("A"))
	}

	if service.GetStock("B") != 5 {
		t.Errorf("Expected stock of B to be 5, got %d", service.GetStock("B"))
	}
}

func TestReserveMultiple_Success(t *testing.T) {
	service := &InventoryService{
		products: map[string]*Product{
			"A": {ID: "A", Name: "Product A", Stock: 10},
			"B": {ID: "B", Name: "Product B", Stock: 10},
		},
	}

	err := service.ReserveMultiple([]ReserveItem{
		{ProductID: "A", Quantity: 5},
		{ProductID: "B", Quantity: 3},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if service.GetStock("A") != 5 {
		t.Errorf("expected stock of A to be 5, got %d", service.GetStock("A"))
	}
	if service.GetStock("B") != 7 {
		t.Errorf("expected stock of B to be 7, got %d", service.GetStock("B"))
	}
}

func TestReserveMultiple_ProductNotFound(t *testing.T) {
	service := &InventoryService{
		products: map[string]*Product{
			"A": {ID: "A", Name: "Product A", Stock: 10},
		},
	}

	err := service.ReserveMultiple([]ReserveItem{
		{ProductID: "A", Quantity: 5},
		{ProductID: "nonexistent", Quantity: 3},
	})
	if !errors.Is(err, ErrProductNotFound) {
		t.Errorf("expected ErrProductNotFound, got %v", err)
	}
	if service.GetStock("A") != 10 {
		t.Errorf("expected stock of A to remain 10, got %d", service.GetStock("A"))
	}
}

func TestReserveMultiple_Concurrent(t *testing.T) {
	// Product A and B each have 50 stock.
	// 100 goroutines each try to reserve 1 of each.
	// Exactly 50 should succeed, final stock for both should be 0 with no oversell.
	service := &InventoryService{
		products: map[string]*Product{
			"A": {ID: "A", Name: "Product A", Stock: 50},
			"B": {ID: "B", Name: "Product B", Stock: 50},
		},
	}

	var wg sync.WaitGroup
	wg.Add(100)
	for i := 0; i < 100; i++ {
		go func() {
			defer wg.Done()
			_ = service.ReserveMultiple([]ReserveItem{
				{ProductID: "A", Quantity: 1},
				{ProductID: "B", Quantity: 1},
			})
		}()
	}
	wg.Wait()

	stockA := service.GetStock("A")
	stockB := service.GetStock("B")

	if stockA < 0 || stockB < 0 {
		t.Errorf("stock went negative: A=%d, B=%d", stockA, stockB)
	}
	if stockA != stockB {
		t.Errorf("stocks are out of sync: A=%d, B=%d", stockA, stockB)
	}
	if stockA != 0 {
		t.Errorf("expected both stocks to be 0, got A=%d B=%d", stockA, stockB)
	}
}

func TestReserveMultiple_ConcurrentWithReserve(t *testing.T) {
	// Simulates production: Reserve and ReserveMultiple hitting the same product simultaneously.
	// 50 goroutines call Reserve(A, 1) and 50 goroutines call ReserveMultiple([{A, 1}]).
	// Total demand equals stock exactly — final stock must be 0 with no oversell.
	service := &InventoryService{
		products: map[string]*Product{
			"A": {ID: "A", Name: "Product A", Stock: 100},
		},
	}

	var wg sync.WaitGroup
	wg.Add(100)
	for i := 0; i < 50; i++ {
		go func() {
			defer wg.Done()
			_ = service.Reserve("A", 1)
		}()
	}
	for i := 0; i < 50; i++ {
		go func() {
			defer wg.Done()
			_ = service.ReserveMultiple([]ReserveItem{
				{ProductID: "A", Quantity: 1},
			})
		}()
	}
	wg.Wait()

	stock := service.GetStock("A")
	if stock < 0 {
		t.Errorf("stock went negative: %d", stock)
	}
	if stock != 0 {
		t.Errorf("expected stock to be 0, got %d", stock)
	}
}

func TestGetStock_ConcurrentWithReserve(t *testing.T) {
	s := &InventoryService{
		products: map[string]*Product{"x": {ID: "x", Name: "x", Stock: 10000}},
	}

	var wg sync.WaitGroup
	for i := 0; i < 500; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.GetStock("x")
		}()
	}
	for i := 0; i < 500; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.Reserve("x", 1)
		}()
	}
	wg.Wait()
}