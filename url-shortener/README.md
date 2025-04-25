# URL Shortener

```
go run .
```

Endpoints: 

- POST /shorten {"url": "https://example.com"} -> http://localhost:9000/{key}
- GET /{key} -> Redirect to the original URL
