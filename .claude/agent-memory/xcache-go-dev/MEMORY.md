# Memory Index — xcache-go-dev

- [Implementazione DeleteByTag (tag index refactor)](project_delete_by_tag.md) — modifiche su tag index set, ErrNotSupported, 3 punti aperti prima del merge
- [Fix TOCTOU in MemoryStore.Get lazy eviction](project_race_condition_get.md) — RISOLTO in commit 927cb62 (2026-04-25): eviction condizionale sotto write lock
