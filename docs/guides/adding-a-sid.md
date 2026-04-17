# Implementare un nuovo backend (Store)

Per aggiungere un nuovo backend (es. Memcached, DynamoDB) basta implementare l'interfaccia `Store`.

!!! note "Unico punto di integrazione"
    L'interfaccia `Store` è l'unico contratto da rispettare. Il resto del sistema (chain, singleflight, generics) non richiede modifiche.

## 1. Implementare `Store`

```go title="store/memcached/store.go"
package memcached

import (
    "context"

    "github.com/bradfitz/gomemcache/memcache"
    xcache "github.com/giulio333/xcache"
)

type MemcachedStore struct {
    client *memcache.Client
}

func NewStore(client *memcache.Client) *MemcachedStore {
    return &MemcachedStore{client: client}
}

func (s *MemcachedStore) Get(ctx context.Context, key string) (any, error) {
    item, err := s.client.Get(key)
    if err == memcache.ErrCacheMiss {
        return nil, xcache.ErrNotFound
    }
    if err != nil {
        return nil, err
    }
    // deserializzare item.Value (JSON, protobuf, ecc.)
    return item.Value, nil
}

// ... implementare Set, Delete, DeleteMany, GetMany, Clear, Close
```

## 2. Rispettare i contratti

| Contratto | Note |
|---|---|
| `Get` restituisce `ErrNotFound` | Mai `nil, nil` per chiavi mancanti |
| `GetMany` omette le chiavi mancanti | La mappa risultante ha solo le chiavi trovate |
| `Close` libera le risorse | Connessioni, goroutine background |
| `Set` rispetta il TTL | Se `opts.TTL > 0`, la chiave deve scadere |

## 3. Usare il nuovo store

```go title="main.go"
mc := memcache.New("localhost:11211")
store := memcached.NewStore(mc)
cache := xcache.New[User](store)
```

Oppure in chain con il memory store:

```go title="main.go"
l1 := memory.NewStore()
l2 := memcached.NewStore(mc)
cache := xcache.New[User](xcache.NewChain(l1, l2))
```

---

*[TTL]: Time To Live
