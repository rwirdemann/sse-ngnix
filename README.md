# sse-ngnix

Statische Seite (nginx) mit Textfeld für JSON, das per Reverse
Proxy an einen lokalen Go-Service weitergeleitet wird.

## Architektur

```
Browser    --GET /------------------> nginx :8080 --> static/index.html
Browser    --POST /api/submit-------> nginx :8080 --> backend :9000/submit
Browser    --GET /confirmations-----> nginx :8080 --> confirmations :9001 (haelt Verbindung offen)
backend    --POST /confirmations----> nginx :8080 --> confirmations :9001 (fuellt offenen Stream)
```

Der `backend`-Service antwortet auf `/submit` sofort mit
`{"status":"accepted", ...}` und simuliert danach 5 Sekunden
Hintergrundarbeit. Parallel dazu öffnet der Browser per
`EventSource` eine `GET /confirmations`-Verbindung; der
`confirmations`-Service hält sie offen, bis `backend` seinen
Abschluss per `POST /confirmations` meldet — erst dann schreibt
`confirmations` das Ergebnis als `text/event-stream` in genau
diese Verbindung zurück. Beide Go-Services sind eigenständige
Module und lauschen nur auf localhost.

Ein Sequenzdiagramm des Ablaufs liegt in
[`docs/sequence-diagram.html`](docs/sequence-diagram.html) (im
Browser öffnen).

## Voraussetzungen

- nginx (`brew install nginx`) — bereits installiert
- Go >= 1.26

## Starten

1. Backend-Service starten:

   ```
   cd backend && go run main.go
   ```

   Lauscht auf `127.0.0.1:9000`.

2. Confirmations-Service starten:

   ```
   cd confirmations && go run main.go
   ```

   Lauscht auf `127.0.0.1:9001`.

3. nginx mit der Projekt-Config starten:

   ```
   nginx -p $(pwd) -c nginx/nginx.conf
   ```

   Lauscht auf `127.0.0.1:8080`.

4. Seite öffnen: http://127.0.0.1:8080

   JSON ins Textfeld eingeben und "Senden" klicken. Die
   sofortige Antwort des Backends erscheint darunter; nach 5
   Sekunden meldet das Backend den Abschluss beim
   Confirmations-Service (sichtbar in dessen Logausgabe).

## Stoppen

```
nginx -p $(pwd) -c nginx/nginx.conf -s stop
```

Beide Go-Services mit Ctrl-C beenden.
