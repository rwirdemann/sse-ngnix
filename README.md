# sse-ngnix

Statische Seite (nginx) mit Formular für eine `ServiceConfig`, die
als Protobuf-Binärnachricht per Reverse Proxy an einen lokalen
Go-Service weitergeleitet wird.

## Architektur

```
Browser  --GET /------------------> nginx :8080 --> static/index.html
Browser  --POST /config/update----> nginx :8080 --> settingsmanager :9000/config/update
```

Sämtliche Nutzdaten sind Protobuf-Nachrichten aus
[`proto/messages.proto`](proto/messages.proto): der Browser
schickt eine `modbus_messages.ServiceConfig` (mit einer `modbus_messages.ModbusConfig` im
`details`-Feld, gepackt als `google.protobuf.Any`) binär an
`/config/update`. Der `settingsmanager`-Service verarbeitet sie
synchron und antwortet direkt mit einer
`modbus_messages.Confirmation{status: received}`. Der Browser dekodiert die
Antwort mit [protobuf.js](https://github.com/protobufjs/protobuf.js),
das die `.proto`-Datei zur Laufzeit über `/proto/messages.proto`
lädt.

## Protobuf-Typen neu generieren

Nach Änderungen an `proto/messages.proto`:

```
cd proto
protoc --proto_path=. --proto_path=<protobuf-include-dir> \
  --go_out=. --go_opt=module=sse-ngnix/proto messages.proto
```

`<protobuf-include-dir>` ist der `include`-Ordner der
protobuf-Installation (z. B. via `brew install protobuf`), der die
Well-known-Types wie `google/protobuf/any.proto` enthält.
Voraussetzung: `protoc` und `protoc-gen-go`
(`brew install protobuf protoc-gen-go`).

## Voraussetzungen

- nginx (`brew install nginx`) — bereits installiert
- Go >= 1.26

## Starten

1. Settingsmanager-Service starten:

   ```
   cd settingsmanager && go run main.go
   ```

   Lauscht auf `127.0.0.1:9000`.

2. nginx mit der Projekt-Config starten:

   ```
   nginx -p $(pwd) -c nginx/nginx.conf
   ```

   Lauscht auf `127.0.0.1:8080`.

3. Seite öffnen: http://127.0.0.1:8080

   JSON-Eingabe im Textfeld anpassen und "Senden" klicken. Die
   Seite wandelt die Eingabe im Browser in eine
   `modbus_messages.ServiceConfig`-Protobuf-Nachricht um, schickt sie binär und
   zeigt die dekodierte `Confirmation`-Antwort direkt darunter an.

## Stoppen

```
nginx -p $(pwd) -c nginx/nginx.conf -s stop
```

Settingsmanager mit Ctrl-C beenden.
