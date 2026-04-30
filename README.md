## Certs

Keypair for server:

```bash
openssl req -x509 -nodes -newkey rsa:4096 -addext "subjectAltName = IP:12.34.56.78" -keyout server.key -out server.cert -sha256
```

Keypair for client:

```bash
openssl req -x509 -nodes -newkey rsa:4096 -keyout client.key -out client.cert -sha256
```
