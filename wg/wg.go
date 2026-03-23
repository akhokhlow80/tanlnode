package wg

import (
	"log"
	"net/netip"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type Service struct {
	ifce string
}

func NewService(interfaceName string) Service {
	return Service{ifce: interfaceName}
}

type Peer struct {
	PublicKey           wgtypes.Key
	PresharedKey        *wgtypes.Key
	Endpoint            *string
	PersistentKeepalive *int64
	AllowedIPs          []netip.Prefix
}

func (s *Service) PutPeer(p *Peer) error {
	args := []string{"set", s.ifce, "peer", p.PublicKey.String()}
	if p.PresharedKey != nil {
		tempFile, err := os.CreateTemp("", "preshared-key")
		if err != nil {
			return err
		}
		defer os.Remove(tempFile.Name())

		b64 := p.PresharedKey.String()
		for len(b64) != 0 {
			n, err := tempFile.Write([]byte(b64))
			if err != nil {
				return err
			}
			b64 = b64[n:]
		}
		if err := tempFile.Sync(); err != nil {
			return err
		}
		args = append(args, "preshared-key", tempFile.Name())
	}
	if p.Endpoint != nil {
		args = append(args, "endpoint", *p.Endpoint)
	}
	if p.PersistentKeepalive != nil {
		args = append(args, "persistent-keepalive", strconv.Itoa(int(*p.PersistentKeepalive)))
	}
	if len(p.AllowedIPs) > 0 {
		var sb strings.Builder
		for i, allowedIP := range p.AllowedIPs {
			sb.WriteString(allowedIP.String())
			if i != len(p.AllowedIPs)-1 {
				sb.WriteRune(',')
			}
		}
		args = append(args, "allowed-ips", sb.String())
	}

	log.Printf("[#] wg %s", strings.Join(args, " "))
	output, err := exec.Command("wg", args...).CombinedOutput()
	if err != nil {
		log.Printf("wg failed: %s: %s", err, output)
		return err
	}

	return nil
}

func (s *Service) RemovePeer(publicKey wgtypes.Key) error {
	args := []string{"set", s.ifce, "peer", publicKey.String(), "remove"}
	log.Printf("[#] wg %s", strings.Join(args, " "))
	output, err := exec.Command("wg", args...).CombinedOutput()
	if err != nil {
		log.Printf("wg failed: %s: %s", err, output)
		return err
	}
	return nil
}
