---
name: Fix TOCTOU in MemoryStore.Get lazy eviction
description: Race condition risolta in store/memory/store.go — eviction condizionale sotto write lock
type: project
---

TOCTOU in `MemoryStore.Get` risolto in commit `927cb62` (2026-04-25).

**Il problema**: `Get` leggeva l'entry sotto RLock, poi acquisiva il write lock per cancellarla se scaduta. Nel gap tra i due lock, un `Set` concorrente poteva rimpiazzare l'entry scaduta con una nuova valida, che veniva poi cancellata incondizionatamente.

**Il fix**: dopo aver acquisito il write lock, si rilegge l'entry e si cancella solo se `expiresAt` è invariato (`current.expiresAt.Equal(it.expiresAt)`).

**Why:** `.Equal()` al posto di `==` per confrontare `time.Time` gestisce correttamente timezone diversi.

**How to apply:** qualsiasi futuro pattern di "leggi sotto RLock, poi scrivi sotto Lock" sullo stesso item deve includere una ri-lettura sotto write lock per evitare lo stesso TOCTOU.
