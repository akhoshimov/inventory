# Backend Developer Assessment - Mid Level (Task 3)

## Instructions

1. Complete all tasks below
2. Push your solution to a **public GitHub repository**

---

## Task: Thread-Safe Inventory Service

Implement a thread-safe inventory management system that handles concurrent stock operations.

### Given: Buggy Implementation

```go
type Product struct {
    ID    string
    Name  string
    Stock int
}

type InventoryService struct {
    products map[string]*Product
}

func (s *InventoryService) GetStock(productID string) int {
    product := s.products[productID]
    if product == nil {
        return 0
    }
    return product.Stock
}

func (s *InventoryService) Reserve(productID string, quantity int) error {
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
    // Check all first
    for _, item := range items {
        product := s.products[item.ProductID]
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
    var mu sync.Mutex
    mu.Lock()
    defer mu.Unlock()

    product := s.products[productID]
    if product.Stock < quantity {
        return ErrInsufficientStock
    }

    product.Stock -= quantity
    return nil
}
```

---

## Your Task

### Part 1: Find All Race Conditions (REVIEW.md)

There are 4 race conditions in the code above. For each one:

```
## Race Condition 1: [Location]
- Code: ...
- What happens: ...
- Production scenario: (specific example with goroutines)
- Fix approach: ...
```

**Hint for #4:** The "SafeReserve" function attempts to fix concurrency but has a fundamental flaw that makes the mutex useless. It's not about using RWMutex.

### Part 2: Implement Thread-Safe Version

Requirements:
- Use `sync.RWMutex` for read/write locking
- Readers don't block each other
- Writers have exclusive access
- `Reserve` must be atomic (check + update in single critical section)
- `ReserveMultiple` must be all-or-nothing

```go
type SafeInventoryService struct {
    mu       sync.RWMutex
    products map[string]*Product
}

func (s *SafeInventoryService) GetStock(productID string) int {
    // TODO: Read lock
}

func (s *SafeInventoryService) Reserve(productID string, quantity int) error {
    // TODO: Write lock, atomic check-and-reserve
}

func (s *SafeInventoryService) ReserveMultiple(items []ReserveItem) error {
    // TODO: All-or-nothing semantics
}
```

### Part 3: Write Concurrent Tests

```go
func TestReserve_ConcurrentOversell(t *testing.T) {
    // Product has 100 stock
    // 200 goroutines each try to reserve 1
    // With race condition: some succeed even when they shouldn't
    // With proper locking: exactly 100 succeed, 100 fail
}

func TestReserveMultiple_Atomicity(t *testing.T) {
    // Product A: 10 stock, Product B: 5 stock
    // Try to reserve 8 of A and 8 of B
    // Should fail entirely (B doesn't have enough)
    // Both A and B should remain unchanged
}
```

---

## Questions - Answer in ANSWERS.md

**Q1:** In `SafeReserve`, the mutex is declared inside the function:
```go
func (s *InventoryService) SafeReserve(...) error {
    var mu sync.Mutex  // <-- HERE
    mu.Lock()
    // ...
}
```

Why does this make the mutex completely useless? Be specific.

**Q2:** Two goroutines call `ReserveMultiple`:
- Goroutine 1: Reserve Product A, then Product B
- Goroutine 2: Reserve Product B, then Product A

If you use per-product locks, what can happen? How do you prevent it?

**Q3:** Look at this "fix":
```go
func (s *SafeInventoryService) Reserve(productID string, quantity int) error {
    s.mu.Lock()
    product := s.products[productID]
    s.mu.Unlock()  // Release early

    if product.Stock < quantity {
        return ErrInsufficientStock
    }

    s.mu.Lock()
    product.Stock -= quantity
    s.mu.Unlock()

    return nil
}
```

Why is this WORSE than having no locks at all? What specific bug does it introduce?

**Q4:** You run tests with `-race` flag and get no warnings. Does this mean your code is race-free? Explain.

---

## Repository Structure

```
your-repo/
├── inventory.go           # Thread-safe implementation
├── inventory_test.go      # Concurrent tests
├── REVIEW.md              # Race condition analysis
└── ANSWERS.md             # Question answers
```

---

## Evaluation

Your submission will be evaluated against our engineering standards document. Key areas:
- Identified all 4 race conditions with correct explanations
- Correct RWMutex usage (RLock for reads, Lock for writes)
- Atomic check-and-reserve (no gap between check and update)
- All-or-nothing semantics for multi-item operations
- Concurrent tests that expose race conditions
- Tests use sync.WaitGroup correctly
