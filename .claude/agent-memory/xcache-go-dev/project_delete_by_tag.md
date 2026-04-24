---
name: Implementazione DeleteByTag (tag index refactor)
description: Stato delle modifiche su cache.go, cache_impl.go, chain.go, store/memory/store.go relative ai tag
type: project
---

Le modifiche (revisionate il 2026-04-24) introducono `DeleteByTag(ctx, tag string) error` su `Store`, `Cache[T]` e `ChainStore`.

**Modifiche chiave:**
- `cache.go`: aggiunta `ErrNotSupported` e metodo `DeleteByTag` all'interfaccia pubblica
- `cache_impl.go`: delega triviale a store
- `chain.go`: itera tutti gli store e chiama `DeleteByTag` su ciascuno (stesso pattern di `Clear`)
- `store/memory/store.go`: indice tag refactored da `map[string][]string` a `map[string]map[string]struct{}` (set O(1) senza duplicati); aggiunto helper `removeFromTagIndex`; pulizia tag integrata in `Delete`, `Set`, `sweep`, lazy eviction in `Get`

**Problemi aperti:**
1. **TOCTOU in Get lazy eviction** — bug preesistente non risolto in questa patch
2. **DeleteByTag non atomica in ChainStore** — se L1 elimina e L2 fallisce, L1 è già mutato (stessa limitazione di `Clear`)
3. **ErrNotSupported definita ma non usata** — serve quando si implementa `DeleteByTag` su RedisStore, oppure va rimossa

**Why:** Refactoring del tag index per eliminare duplicati e migliorare le performance di lookup.
**How to apply:** Quando si riprende il lavoro su tag/DeleteByTag, verificare questi tre punti prima del merge.
