# Import tips

## Tricks
to import tricks, you can use the following command from the commandline:

### On development environment
```bash
    curl \
        -w '%{http_code} %{url}\n' \
        -X PUT \
        -H 'Content-Type: application/json' \
        -d @$i \
        "http://localhost:8000/tricks/$(jq -r '.name|@uri' $i)"
```

### On production environment
note: take the bearer from https://api.acroworldtour.com/docs/
```bash
for i in tricks/2023/*.json; do
    curl \
        -w '%{http_code} %{url}\n' \
        -X PUT \
        -H 'Authorization: Bearer xxx' \
        -H 'Content-Type: application/json' \
        -d @$i \
        "https://api.acroworldtour.com/tricks/$(jq -r '.name|@uri' $i)"
done
```
