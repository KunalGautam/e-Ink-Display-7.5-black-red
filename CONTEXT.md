# System Architecture & Design Decisions

This document describes the architectural design and implementation details of the Go-Python E-Ink Dashboard system, incorporating complex text shaping for Devanagari (Hindi/Marathi) scripts, 7-day calendar schedule syncing, a web management portal, and SQLite persistence.

## System Architecture Diagram

```mermaid
flowchart TD
    subgraph Sources [Data Inputs]
        M1[Home Assistant / Scripts] -- Notes JSON --> Broker
        M2[Mail Watcher / Scripts] -- Emails JSON --> Broker
        M3[Task Scheduler / Scripts] -- MQTT Calendar JSON --> Broker
        GCal[Google Calendar Service] -- Private iCal Feed --> ICalWorker
    end

    subgraph Broker [MQTT Message Broker]
        B[MQTT Broker / e.g. Mosquitto]
    end

    subgraph Server [Go API Server]
        subgraph DB [SQLite Database]
            SQL[(epaper.db)]
        end

        subgraph MQTT [MQTT Client]
            C[Subscription Client] --> |Thread-Safe Write| Cache[(MQTT Cache)]
            C --> |Persist Last 10| SQL
        end
        
        subgraph ICal [iCal Sync Worker]
            ICalWorker[ICS Puller & Parser] --> |15-Min Sync| GCalCache[(Google Cal Cache)]
        end
        
        subgraph HTTP [HTTP REST API]
            H1["/image (PNG)"] --> R[Command Line delegator]
            H2["/image/raw (Raw Bytes)"] --> R
            H3["/settings (Web GUI Form)"] --> |Basic Auth / bcrypt| SQL
            H4["/api/settings (JSON)"] --> |Basic Auth / bcrypt| SQL
            H5["/restart (Shutdown Process)"] --> |Basic Auth / bcrypt| SQL
            R --> |Merge & Sort Events| Merged[(Schedule Cache)]
            Merged --> |Notes + Emails + Schedule| PY[Python renderer.py helper]
            PY --> |Poppins Font + Pillow Harfbuzz| Output[temp_layout.png]
            Output --> R
            R --> P[Pixel Buffer Packer]
        end
        
        SQL --> |Load settings on startup| HTTP
        SQL --> |Pre-populate notes/emails caches| Cache
    end

    subgraph Admin [Web Administrator]
        Browser[Web Browser] --> |GET /settings (Basic Auth)| H3
        Browser --> |Change configurations & password| H3
        Browser --> |Trigger graceful shutdown| H5
        H3 --> |Live Preview img src| H1
    end

    subgraph Client [Raspberry Pi Client]
        PC[Python Client Loop] --> |HTTP GET /image| H1
        PC --> |SHA-256 Hash Compare| Check{Has data changed?}
        Check --> |No & < 24h| Sleep[Skip display refresh]
        Check --> |Yes / Or >= 24h| Display[Full refresh panel]
        Display --> |Pillow Split Colors| E[SPI Display Driver]
        E --> |SPI Hardware Interface| HW[Waveshare 7.5 e-Ink Screen]
    end

    Broker --> |Retained Message Stream| C
```

---

## Data Flow Details

1. **Database-Driven Settings**: On startup, the Go server initializes `epaper.db` (creating tables if not exists) and reads configuration fields. If the database is empty, it populates it with values from `config.yaml` or environment variables so they can be edited dynamically in the settings GUI.
2. **Basic Auth and Web GUI**: The `/settings` and `/api/settings` routes are protected by Basic Access Authentication. Authentication hashes are stored in SQLite using `bcrypt`. The settings form lets administrators change MQTT topics, broker endpoints, timezones, and display dimensions. It features a live layout preview pane and a system shutdown trigger.
3. **Google Calendar (ICS URL) Feed**: A Go background sync worker wakes up every 15 minutes, sends an HTTP GET request to the Google Calendar secret ICS URL, parses the calendar entries, filters for events occurring in the next 7 days, and updates the local cache.
4. **On-Demand Merge and Layout Generation**: When the `/image` or `/image/raw` endpoint is polled:
   - Calendar events from MQTT and Google Calendar are combined and sorted chronologically.
   - The Go server calculates the maximum timestamp of the last database update or sync, formatting it as a static `"last_updated"` value.
   - The Go server executes `python3 ./renderer.py` via `os/exec.Command`, feeding the notes, emails, schedule, and static timestamp on stdin.
   - The Python script uses **Pillow** to draw the layout, utilizing the **Poppins** font family and Pillow's native Raqm shaping engine to render Devanagari text perfectly. It saves the visual output as a temporary PNG.
   - The Go server decodes the PNG and processes it (either returning the PNG directly or converting it to 1-bit monochrome buffers for the e-ink screen).
5. **Client-Side Skip-Refresh**: The Python client fetches the image, calculates its SHA-256 hash, and compares it to a local cache stored in `.client_state.json`. If the hash is identical and less than 24 hours have elapsed (anti-ghosting safety threshold), it skips the slow, high-wear hardware refresh.

---

## Key Design Decisions

### 1. Python rendering helper for Devanagari Support
While Go is ideal for orchestrating network connections, caching, and serving HTTP requests, Go's standard font rasterization libraries (`freetype`) lack a text shaper (like HarfBuzz). Without a shaper, Devanagari vowel markers (*matras*) and conjuncts render disjointed and unreadable. 
* By delegating the 2D layout composition to a lightweight Python helper script that utilizes Pillow (which compiles with HarfBuzz/FriBidi support via Raqm), we get **100% perfect Devanagari rendering** with Poppins font while keeping the Go server as the central daemon.

### 2. Time-Interval ICS Overlap Checking
Google Calendar feeds include multi-day, single-day, and specific time-bound events. To ensure we show events relevant to the next 7 days, the Go parser computes the date boundaries (`00:00:00` today to `23:59:59` 7 days from now in the configured timezone) and checks for interval overlaps:
$$eventStart < periodEnd \quad \text{and} \quad eventEnd > periodStart$$

### 3. Chronological Sorted Schedule
Rather than making the user choose between Google Calendar and MQTT schedules, we support both concurrently. Events are merged into a single schedule list, sorted by their time strings (ensuring "All Day" banners are at the top and timed meetings are listed chronologically), and capped to prevent visual overflow in the schedule column.

### 4. Pure Go SQLite and bcrypt
To keep compilation 100% portable and easy to run on targets like Raspberry Pi without a GCC C toolchain, we use `modernc.org/sqlite` (Cgo-free) rather than `mattn/go-sqlite3`. Access credentials are securely stored using standard salted `bcrypt` hashes.

### 5. Offline Resiliency Cache
MQTT messages are not guaranteed to be retained, especially on broker restarts or network resets. To guarantee the dashboard displays historical notes and emails on server restart, the Go MQTT client saves the last 10 notes and emails in SQLite tables (pruned using autoincrement ID limits) and reads them back into memory on startup.
