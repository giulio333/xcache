# Ciclo di vita dei SID

Un SID attraversa due fasi distinte: **persistenza** e **runtime**.

## Fase 1 — Persistenza (DB)

La tabella `sids` è la fonte di verità su quali SID il sistema deve gestire:

```sql title="Schema tabella sids"
CREATE TABLE sids (
    id     TEXT PRIMARY KEY,
    name   TEXT NOT NULL,
    type   TEXT,
    config TEXT NOT NULL DEFAULT '{}'
);
```

Il campo `config` contiene un JSON arbitrario con la configurazione specifica del provider. Questo garantisce che nessuna informazione vada persa durante il round-trip DB → oggetto.

!!! warning "Differenza con i vecchi design"
    In precedenza la tabella salvava solo `id`, `name` e `type`. Qualsiasi configurazione extra (URL, credenziali) veniva persa al riavvio. Il campo `config` risolve questo problema.

## Fase 2 — Runtime (registry in-memory)

Al bootstrap, ogni record nel DB viene istanziato come oggetto vivo:

```go title="manager/sid_manager.go"
func (sm *SidManager) Bootstrap() error {
    saved, err := sm.repo.FindAllSids()
    // ...
    for _, dto := range saved {
        provider := sm.instantiate(dto) // (1)
        sm.registry[dto.ID] = provider  // (2)
    }
}
```

1. Crea un oggetto `SidInterface` concreto dal DTO
2. Lo aggiunge al registry in-memory — da qui in poi mai più letto dal DB

Dopo il bootstrap, `GetSIDs()` legge sempre dalla mappa in-memory, **mai** dal DB.

## Registrazione di un nuovo SID

`AddSid()` esegue entrambe le operazioni in sequenza:

```go title="core/core.go"
func (c *Core) AddSid(s sid.SidInterface) {
    if err := c.sidManager.RegisterSid(s); err != nil { // (1)
        c.logger.Error("registrazione SID fallita", ...)
    }
}
```

1. `RegisterSid` salva sul DB **e** aggiunge al registry in-memory — il SID è immediatamente attivo

## Interfaccia SidInterface

Ogni provider deve implementare i seguenti metodi:

| Metodo | Tipo ritorno | Descrizione |
|---|---|---|
| `GetID()` | `string` | Identificatore univoco |
| `GetName()` | `string` | Nome leggibile |
| `GetType()` | `string` | Tipo stringa (es. `#!go "hanwha"`) usato per deserializzare dal DB |
| `GetConfig()` | `map[string]any` | Configurazione serializzabile in JSON |
| `GetDevices()` | `[]Device` | Discovery dei dispositivi — bloccante |
| `StartEventStream()` | — | Stream continuo di eventi — bloccante |

!!! note
    `GetType()` è il discriminatore usato da `SidManager.instantiate()` per capire quale struct concreta creare a partire dal record DB. È l'unico punto del codebase che conosce i tipi concreti dei SID.

---

*[SID]: Source of Devices
*[DB]: Database
