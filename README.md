```bash
umask 077

NODE="<your node name>"
mkdir -p /etc/tanlnode/$NODE
mkdir -p /var/lib/tanlnode/$NODE

# === Install

./install.sh

# === Certs

SUBJ="DNS:domain-name.com, IP:1.2.3.4"

openssl req -x509 -nodes -newkey rsa:4096 -addext "subjectAltName = $SUBJ" \
   -days 180 -subj "/" -keyout /etc/tanlnode/$NODE/server.key \
   -out /etc/tanlnode/$NODE/server.cert -sha256

touch /tmp/tanlnode-client.key
openssl req -x509 -nodes -newkey rsa:4096 -keyout /tmp/tanlnode-client.key \
   -days 180 -subj "/" -out /etc/tanlnode/$NODE/client.cert -sha256

# === Configure

cp example.conf /etc/tanlnode/$NODE/config
$EDITOR /etc/tanlnode/$NODE/config # alter config
systemctl enable --now tanlnode@$NODE.service
```
