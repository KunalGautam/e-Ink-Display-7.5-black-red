# E-Ink Dashboard Architecture

This document describes the design rationale, data flow, widget extensibility, and deployment topologies of the multi-canvas widget-based e-Paper platform.

---

## 1. System Topology

The platform follows a decoupled client-server architecture:

```
[MQTT Sources / n8n]       [Google Calendar ICS]
         │                            │
         ▼ (Publish / Sync)           ▼ (Fetch / Cache)
┌───────────────────────────────────────────────┐
│              Go REST API Server               │
│                                               │
│  ┌──────────────────┐   ┌──────────────────┐  │
│  │   SQLite Store   │   │  MQTT Sub Cache  │  │
│  │ (Canvas/Widgets) │   │ (Thread-Safe Registry)│
│  └────────┬─────────┘   └────────┬─────────┘  │
│           │                      │            │
│           ▼                      ▼            │
│     ┌──────────────────────────────────┐      │
│     │    Pure Go Rendering Pipeline    │      │
│     │  (Sub-contexts per Widget / gg)  │      │
│     └────────────────┬─────────────────┘      │
└──────────────────────┼────────────────────────┘
                       │
                       ▼ (HTTP GET /canvas/{id}/render)
┌───────────────────────────────────────────────┐
│           Raspberry Pi Client Daemon          │
│  - Reads client_config.json                   │
│  - Decodes packed bits                        │
│  - Controls waveshare_epd driver via SPI      │
└───────────────────────────────────────────────┘
```

---

## 2. Core Rendering Pipeline (Pure Go)

Unlike the old single-canvas server that spawned Python scripts, this implementation executes composition natively in Go using the `github.com/fogleman/gg` 2D vector library.

```
       Canvas Render Request
                 │
                 ▼
     Read Canvas Profile (DB)
                 │
                 ▼
      Fetch Widget Configuration
                 │
                 ▼
       For each Widget:
  ┌────────────────────────────────────────────────────────┐
  │ 1. Create sub-context sized to Widget Width x Height   │
  │ 2. Load and cache Google Font face (truetype)          │
  │ 3. Fetch cached MQTT payload if topic binding exists   │
  │ 4. Call widget.Render(subCtx)                          │
  │ 5. Composite sub-context onto main Canvas at (X, Y)    │
  └────────────────────────────────────────────────────────┘
                 │
                 ▼
   Separate Black/Red Channels
                 │
                 ▼
     Pack Bits (1 byte = 8px)
                 │
                 ▼
      application/octet-stream
```

---

## 3. Dynamic MQTT Cache Registry

To allow independent topic bindings per widget without spawning redundant MQTT clients, the server employs a thread-safe connection and cache pooling registry (`internal/mqtt/registry.go`).

1. **Lazy Subscriptions**: When a widget is rendered or saved, if its topic is not yet active, the registry establishes a client to the target broker URL (persisting connections in a map) and issues a subscriber callback.
2. **Payload Stashing**: When a message lands on a subscribed topic, the registry parses the raw payload and updates an in-memory cache keyed by `brokerURL + "||" + topic`.
3. **Widget Fetch**: When the widget draws, it retrieves the stashed payload string and parses it (e.g. JSON weather details, plain string notes).

---

## 4. Extensibility Guide: Adding New Widget Types

The platform is designed around a pluggable, interface-based architecture. To add a new widget type:

1. **Define the Renderer**: Create a new Go source file under [go-server/internal/widget/](file:///home/kunal/Projects/epaper-display/go-server/internal/widget/) (e.g. `stock_prices.go`):
   ```go
   package widget

   import (
       "context"
       "encoding/json"
   )

   type StockWidget struct{}

   func (w *StockWidget) Render(ctx context.Context, rCtx *RenderContext) error {
       rCtx.Ctx.SetHexColor(rCtx.ColorFG)
       // Draw standard shapes, strings, or wrapped blocks using rCtx.Ctx (gg.Context)
       rCtx.Ctx.DrawString("AAPL: $240.50", 10, 20)
       return nil
   }
   ```

2. **Register the Type**: Add your type mapping inside `RenderCanvas` in [go-server/internal/canvas/canvas.go](file:///home/kunal/Projects/epaper-display/go-server/internal/canvas/canvas.go):
   ```go
   switch w.Type {
   case "calendar":
       wRenderer = &widget.CalendarWidget{}
   // ...
   case "stocks":
       wRenderer = &widget.StockWidget{}
   }
   ```

3. **Configure in Admin Web**: Add options or custom placeholder variables to the template selection UI in [settings.html](file:///home/kunal/Projects/epaper-display/go-server/static/settings.html).
