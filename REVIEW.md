## Review

As stated in the doc, there are 4 bugs in the implementation, after observations, I found out that each method on inveotory service has concurrency issues, 3 of them mainly the same issue but the last method safe reserve has a different type of concurrency issue. In this document I will try to explain each case and how to fix.

> [!NOTE]
> **TOCTOU (Time-of-Check-Time-of-Use):** A concurrency bug where the state you checked is no longer valid by the time you act on it, because another goroutine modified it in between.

---

## Race Condition 1: GetStock — unprotected map read
```go
func (s *InventoryService) GetStock(productID string) int {
    product := s.products[productID]  // no lock
    ...
    return product.Stock              // no lock
}
```
- What happens: The map `s.products` is read without any lock. In Go, concurrent map reads and writes are prohibited, meaning it the app will crash. 
- Production scenario: Goroutine A calls GetStock("product A") while Goroutine B calls Reserve("product A", 5) at the same time. Both hit the map simultaneously — Go's race detector fires and the program      
  crashes.
- Fix approach: We need to acquire `s.mu.RLock()` before reading the map and release it `s.mu.RUnlock()` after we are done with manipulating or reading. Good thing is multiple readers are allowed to hold RLock simultaneously (at the same time), thereby we can say this doesn't block concurrent GetStock calls.

---

## Race Condition 2: Reserve

```go
func (s *InventoryService) Reserve(productID string, quantity int) error {
    product := s.products[productID]   // no lock

    if product.Stock < quantity {       // check happens here
        return ErrInsufficientStock
    }

    product.Stock -= quantity           // update happens here — gap between check and update
    return nil
}
```
- What happens:  Here we have two operations first check and then update, basically two operations, and we need to be careful with checking and updating the map concurrently without locking it because right after it runs check some other goroutine may have already updated the map thereby making the map check irrelevant, making the map stale. 
- Production scenario: Stock = 10. Goroutine A and Goroutine B both try to reserve 8 units. Both read Stock = 10, both pass the check (10 >= 8). Goroutine A decrements first → Stock = 2. Then Goroutine
  B decrements → Stock = -6. You've sold 6 units you don't have. 
- Fix approach: Hold a write lock (s.mu.Lock()) for the entire check-and-update operation, so that any reads and writes to the map is going to wait the parallel (simultaneous) updates or reads

---

## Race Condition 3: ReserveMultiple — two-phase design creates a TOCTOU window

```go
func (s *InventoryService) ReserveMultiple(items []ReserveItem) error {
    // Check all first
    for _, item := range items {
        product := s.products[item.ProductID]  // no lock
        if product.Stock < item.Quantity {
            return ErrInsufficientStock
        }
    }

    // Then reserve all
    for _, item := range items {
        s.products[item.ProductID].Stock -= item.Quantity  // no lock
    }

    return nil
}
```
- What happens: There are two separate loops with no lock held across either of them. The gap between the check loop and the update loop allows other goroutines to drain stock after the checks have passed but before the updates run. The all-or-nothing guarantee is completely broken.
- Production scenario: Product A has Stock = 5. Goroutine 1 calls ReserveMultiple([{A, 5}]) — passes the check (5 >= 5). Before the update loop runs, Goroutine 2 calls Reserve("A", 3) and decrements Stock to 2. Now Goroutine 1's update loop runs and decrements by 5 → Stock = -3. The check result was valid but became stale before it was already updated by another goroutine operation.
- Fix approach: Hold a single write lock (s.mu.Lock()) across both loops — the validation phase and the update phase must be one uninterrupted critical section to support all-or-nothing.

---

## Race Condition 4: SafeReserve — local mutex is completely useless

```go
func (s *InventoryService) SafeReserve(productID string, quantity int) error {
    var mu sync.Mutex  // <-- local variable: new mutex created per call
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
- What happens: Here, it's a bit different issue but still it's a concurrency problem, `mu` is declared as a local variable inside the function. Every goroutine that calls SafeReserve creates its own brand-new, independent sync.Mutex. Since each goroutine locks its own private mutex, there is zero contention, all goroutines proceed simultaneously without ever blocking each other. The lock provides no mutual exclusion whatsoever.
- Production scenario: Goroutine A and Goroutine B both call SafeReserve("SKU-1", 8) when Stock = 10. Each creates its own `mu`, each locks its own `mu` without waiting, and both proceed into the critical section at the same time — identical oversell outcome to Race Condition 2, despite the lock being present.
- Fix approach: The mutex must be a shared field on the struct (s.mu sync.RWMutex) so that all goroutines compete for the same lock. A lock only provides mutual exclusion when all contending goroutines refer to the same mutex instance.


---
> [!NOTE]                                                                                                                                                                                                
> All reported concurrency bugs have been found after writing comprehensive tests on inventory_test.go file, feel free to review it for further details
