```bash
WG_IF="<your wg interface name>"
mkdir -p /etc/tanlnode/$WG_IF
chmod 600 /etc/tanlnode/$WG_IF
mkdir -p /var/lib/tanlnode/$WG_IF
chmod 600 /var/lib/tanlnode/$WG_IF

# === Install

./install.sh

# === Certs

SUBJ="DNS:domain-name.com, IP:1.2.3.4"

openssl req -x509 -nodes -newkey rsa:4096 -addext "subjectAltName = $SUBJ" \
   -days 180 -subj "/" -keyout /etc/tanlnode/$WG_IF/server.key \
   -out /etc/tanlnode/$WG_IF/server.cert -sha256

touch /tmp/tanlnode-client.key
chmod 600 /tmp/tanlnode-client.key
openssl req -x509 -nodes -newkey rsa:4096 -keyout /tmp/tanlnode-client.key \
   -days 180 -subj "/" -out /etc/tanlnode/$WG_IF/client.cert -sha256

# === Configure

cp example.conf /etc/tanlnode/$WG_IF/config
$EDITOR /etc/tanlnode/$WG_IF/config # alter config
systemctl enable --now tanlnode@$WG_IF.service
```
