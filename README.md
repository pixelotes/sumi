# sumi

Minimal, filesystem-first EPUB reader. The third in the suika / kaze / sumi
triplet: same philosophy, different domain.

- No database. The server doesn't track anything per-user.
- Directory navigation in the web UI (mirrors suika / kaze).
- Cover priority: sibling image (`My Book.jpg` next to `My Book.epub`) →
  folder cover (`cover.jpg` / `folder.jpg` / `book.jpg`) → embedded cover
  extracted from the EPUB.
- Reader is [epub.js](https://github.com/futurepress/epub.js) rendered
  client-side. The server only streams the raw `.epub`.
- Reading position is stored in the **browser's localStorage** — no
  server-side state, no SQLite. The downside: bookmarks don't roam across
  devices.

## Layout expected

```
books/
├── Author Name/
│   ├── Book Title.epub
│   └── Book Title.jpg     (optional sibling cover)
└── Another Folder/
    ├── cover.jpg          (folder-wide cover)
    └── *.epub
```

Folder structure is just for browsing; the server doesn't read any of it
as metadata.

## Configuration

`config/config.yml`:

```yaml
app:
  port: 8085
  ui_password: "changeme"
  jwt_secret: "change-me-in-production"

libraries:
  - friendly_name: "Books"
    path: "./books"
```

## Run

```
go run ./cmd/sumi -config config/config.yml
```

Or with Docker:

```
docker build -t sumi .
docker run -p 8085:8085 \
  -v ./config:/app/config:ro \
  -v /path/to/books:/app/books:ro \
  sumi
```

Open `http://localhost:8085`, log in with the password from the config.
Click into a folder, click an EPUB to read. Arrow keys or click on the
left/right zones to turn pages. Esc returns to the library.
