# Flusso degli eventi

Gli eventi percorrono un percorso asincrono dal SID al database.

## Pipeline completa

```
SID.StartEventStream()
        │
        │  chan event.Event  (per SID, buffer 10)
        ▼
  goroutine listener (Core)
        │
        │  EventManager.IngestEvent()
        ▼
  canale interno EventManager  (buffer 100)
        │
        │  goroutine listenAndProcess()
        ▼
  repo.SaveEvent()  ──►  tabella events (SQLite)
```

## Canali e buffer

| Canale | Buffer | Creato da | Rischio di drop |
|---|---|---|---|
| Per-SID (`eventChan`) | 10 | `Core.ListenEvents()` | Basso — listener sempre attivo |
| Interno `EventManager` | 100 | `NewEventManager()` | Medio — se DB è lento |

!!! warning "Drop silenzioso"
    Se il canale interno è pieno, l'evento viene **scartato** senza essere persistito. Il sistema logga un `WARN` ma non fa retry. In produzione, aumentare il buffer o aggiungere un meccanismo di backpressure.

## Struttura di un evento

```go title="event/event.go"
type Event struct {
    DeviceID  string
    SidID     string
    Code      string    // es. "VIDEO_LOSS", "DISK_FULL"
    Value     any       // bool, float, stringa o JSON arbitrario
    Timestamp time.Time
}
```

Il campo `Value` viene serializzato in JSON prima della persistenza (`#!go json.Marshal`) e deserializzato alla lettura (`#!go json.Unmarshal`).

## Codici evento per tipo device

| Device | Codice | Tipo `Value` |
|---|---|---|
| `CAMERA` | `VIDEO_LOSS` | `bool` |
| `DVR` | `DISK_FULL` | `bool` |

## Consultare lo storico

`EventManager` espone due metodi di query:

`GetRecentEvents(limit int)`
:   Ultimi N eventi, ordinati per `timestamp DESC`. Usato dalla dashboard.

`GetDeviceHistory(deviceID, sidID string)`
:   Storico completo di un singolo device su un SID specifico. Filtra su entrambe le colonne perché la PK di `sensors` è la coppia `(id, sid_id)`.

---

*[SID]: Source of Devices
*[DB]: Database
*[PK]: Primary Key
