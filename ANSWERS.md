
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

> The reason why this mutex is completely useless is it's declared inside the method meaning every call to this method creates its own mutex instance, instead of having a single mutex source, 
> a mutex only provides mutual exclusion when all contending goroutines compete for the same instance — since each goroutine gets its own private mutex, none of them ever block each other, making the
> lock completely ineffective.

**Q2:** Two goroutines call `ReserveMultiple`:
- Goroutine 1: Reserve Product A, then Product B
- Goroutine 2: Reserve Product B, then Product A

If you use per-product locks, what can happen? How do you prevent it?

> In the current implementation this does not cause any deadlocks or any concurrency related bug, because we are locking entire map to update the stock, the whole inventory.
> But if we want to use per-product locks like
- Goroutine 1: Reserve Product A, then Product B
- Goroutine 2: Reserve Product B, then Product A
> This will create a deadlock, because Goroutine already took a lock on Product A and Goroutine 2 is already took a lock on Product B, and each Goroutine is trying to take a lock on locked products which creates deadlock.
> To prevent it, always acquire per-product locks in a **consistent order** — for example, sorted by product ID. If both goroutines always lock the lower ID first, Goroutine 2 would try to lock A before B, so it blocks waiting for Goroutine 1 to release A instead of deadlocking.

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

> This is worse than no locks because it creates a **false sense of security**. The TOCTOU bug is still present — the check `product.Stock < quantity` runs outside any lock, so another goroutine can drain the stock between the check and the update, causing oversell. But unlike having no locks at all, this version *looks* correct to a reviewer and the race detector may stay silent since each individual memory access is behind a lock. No locks would at least fail loudly; this fails silently in production.

**Q4:** You run tests with `-race` flag and get no warnings. Does this mean your code is race-free? Explain.
> No, it's not race free for sure. The `-race` flag only detects **data races** — concurrent unsynchronized accesses to the same memory location. It does not detect **logical races** like TOCTOU, where every individual read and write is behind a lock but the overall check-and-act sequence is not atomic. The Q3 "fix" is a perfect example: each access is locked so the race detector stays silent, yet the oversell bug is still there. Clean `-race` output means no data races were observed during that run — not that the logic is correct.
---