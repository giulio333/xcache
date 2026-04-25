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

func (s *MemcachedStore) Get(ctx context.Context, key string) (xcache.Entry, error) {
    item, err := s.client.Get(key)
    if err == memcache.ErrCacheMiss {
        return xcache.Entry{}, xcache.ErrNotFound
    }
    if err != nil {
        return xcache.Entry{}, err
    }
    // deserializzare item.Value (JSON, protobuf, ecc.)
    return xcache.Entry{Value: item.Value}, nil
}

// ... implementare Set, Delete, DeleteMany, GetMany, Clear, DeleteByTag, Close
```

## 2. Operazioni opzionali e `ErrNotSupported`

Se il backend non può implementare in modo efficiente un'operazione opzionale (es. `DeleteByTag` su un Memcached senza indice tag dedicato), restituire `xcache.ErrNotSupported`:

```go title="store/memcached/store.go"
func (s *MemcachedStore) DeleteByTag(ctx context.Context, tag string) error {
    return xcache.ErrNotSupported
}
```

I chiamanti possono distinguere il caso "feature non disponibile" da un errore di trasporto con `errors.Is(err, xcache.ErrNotSupported)`.

## 3. Rispettare i contratti

| Contratto | Note |
|---|---|
| `Get` restituisce `ErrNotFound` | Mai `Entry{}, nil` per chiavi mancanti |
| `Get` popola `Entry.ExpiresAt` e `Entry.Tags` | Necessario per propagare TTL e tag nella chain cache |
| `GetMany` omette le chiavi mancanti | La mappa risultante ha solo le chiavi trovate |
| `Close` libera le risorse | Connessioni, goroutine background |
| `Set` rispetta il TTL | Se `opts.TTL > 0`, la chiave deve scadere |
| `Set` aggiorna l'indice tag su overwrite | Se il backend ha un indice tag, evitare riferimenti stale |
| `DeleteByTag` non implementato | Restituire `ErrNotSupported`, non `nil` |

## 4. Usare il nuovo store

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
