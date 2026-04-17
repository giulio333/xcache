Documento di Specifica Architetturale

Nome Progetto: XCache (Nome provvisorio)
Linguaggio: Go 1.21+ (per sfruttare le ultime ottimizzazioni dei generics e del garbage collector).

1. Obiettivi e Visione (Core Tenets)

Type-Safety Assoluta: L'utente inserisce una struct User e recupera una struct User senza mai fare un type assertion.

Zero-Config per iniziare: Funziona subito in memoria locale con una singola riga di codice, ma scala su cluster Redis quando necessario.

Estendibilità Trasparente: Metriche, logging e meccanismi di "fallback" (L1/L2 cache) non devono inquinare il codice core, ma essere applicabili come livelli (layers).

2. Pattern Design Adottati

Strategy Pattern: Il core della cache delegherà il salvataggio fisico dei dati a componenti intercambiabili (MemoryStore, RedisStore, MemcachedStore).

Functional Options Pattern: Il modo idiomatico in Go per passare configurazioni opzionali (es. TTL, Tag) senza creare struct di configurazione enormi o costruttori con 10 parametri.

Decorator / Middleware Pattern: Per implementare la Chain Cache (es. cerca in memoria -> se fallisce cerca in Redis), Loadable Cache (se non c'è, caricalo dal DB) e l'Osservabilità (Prometheus).

3. Specifiche dell'Interfaccia Core

L'interfaccia deve essere minimale. Evitiamo di inserire metodi specifici di un database (come i comandi HASH di Redis) nel core.

Go
package xcache

import (
	"context"
	"time"
)

// Item definisce la struttura base per le opzioni (Functional Options)
type Options struct {
	TTL  time.Duration
	Tags []string
}

type Option func(*Options)

// Funzioni helper per le opzioni
func WithTTL(d time.Duration) Option { return func(o *Options) { o.TTL = d } }
func WithTags(tags ...string) Option { return func(o *Options) { o.Tags = tags } }

// Store è il contratto che ogni backend (Memoria, Redis, ecc.) deve rispettare.
// È slegato dal tipo di dato (usa any) perché lo Store gestisce bytes/interfacce,
// mentre il wrapper Cache gestisce i Generics.
type Store interface {
	Get(ctx context.Context, key string) (any, error)
	Set(ctx context.Context, key string, value any, opts ...Option) error
	Delete(ctx context.Context, key string) error
	Clear(ctx context.Context) error
}

// Cache è l'interfaccia generica esposta all'utente.
type Cache[T any] interface {
	Get(ctx context.Context, key string) (T, error)
	Set(ctx context.Context, key string, value T, opts ...Option) error
	// GetOrLoad implementa il pattern "Loadable": se manca, chiama la funzione 'loader'
	GetOrLoad(ctx context.Context, key string, loader func() (T, error), opts ...Option) (T, error)
}
4. Specifica di Developer Experience (DX)

Ecco come un utente dovrebbe interagire con la libreria. L'obiettivo è un'API pulita, che si legge come una frase:

Go
// Setup in una riga per la memoria locale (con default sensati)
myCache := xcache.New[User](memory.NewStore())

// Setup avanzato: Chain (Memoria -> Redis) con Metriche
l1 := memory.NewStore(memory.WithShards(64))
l2 := redis.NewStore(redisClient)
chainedStore := xcache.NewChain(l1, l2)

// Decorator per le metriche applicato sopra allo store
metricStore := prometheus.Wrap(chainedStore)

// Inizializzazione della cache tipizzata
userCache := xcache.New[User](metricStore)

// Utilizzo semplice usando GetOrLoad (Previene il problema della "Cache Stampede")
user, err := userCache.GetOrLoad(ctx, "user:123", func() (User, error) {
    return database.FindUserByID(123)
}, xcache.WithTTL(10 * time.Minute))
5. Scelte Architetturali per l'Efficienza

Object Pooling: L'uso di sync.Pool per i buffer di byte usati durante la serializzazione (JSON/Protobuf) verso Redis. Questo riduce drasticamente la pressione sul Garbage Collector.

Eviction Attiva e Passiva (per l'in-memory): * Passiva: Il dato viene controllato per la scadenza solo quando viene letto (Get).

Attiva: Una goroutine in background "spazza" i dati scaduti periodicamente per evitare Out Of Memory (OOM).

Singleflight: Implementazione obbligatoria nel metodo GetOrLoad (usando golang.org/x/sync/singleflight). Se 10.000 richieste per la chiave "X" arrivano contemporaneamente e la chiave non c'è, solo una query andrà al database, le altre 9.999 aspetteranno il risultato della prima. Questo previene il collasso del database (Cache Stampede).